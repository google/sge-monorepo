// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include <cassert>
#include <chrono>
#include <deque>
#include <iostream>
#include <memory>
#include <mutex>
#include <vector>

#include "p4_cgo_bridge.h"

#include "p4/clientapi.h"

// Try not to keep more than this many clients idle.
static const int kMaxClients = 16;

class Pool {
public:
  std::shared_ptr<ClientApi> Client(int* ns, std::string* error, bool* fresh) {
	const auto start = std::chrono::high_resolution_clock::now();
	// Manipulate the queue of ready clients under fine grained locks.
	std::unique_ptr<ClientApi> client;
	{
	  std::lock_guard<std::mutex> lock(mu_);
	  if (!clients_.empty()) {
		client = std::move(clients_.front());
		clients_.pop_front();
	  }
	}
	if (!client) {
	  client.reset(new ClientApi());
	  client->SetCharset("utf8");
	  SetProtocol(client.get());

	  Error err;
	  client->Init(&err);
	  
	  if (err.Test()) {
		StrBuf msg;
		err.Fmt(&msg);
		if (error) {
		  *error = "error initializing client: ";
		  *error += msg.Text();
		} else {
		  std::cerr << "error initializing client: " << msg.Text() << std::endl;
		}
		return nullptr;
	  }
	  *fresh = true;

	  if (ns) {
		const auto stop = std::chrono::high_resolution_clock::now();
		*ns += (stop - start).count() / 1000;
	  }
	}
	
	auto deleter = [this](ClientApi* c) {
					 std::lock_guard<std::mutex> lock(mu_);
					 if (!c->Dropped() && (clients_.size() < kMaxClients)) {
					   clients_.push_back(std::unique_ptr<ClientApi>(c));
					 } else {
					   delete c;
					 }
				   };
	return std::shared_ptr<ClientApi>(client.release(), deleter);
  }

protected:
  virtual void SetProtocol(ClientApi* c) {}
  
private:
  using ClientQueue = std::deque<std::unique_ptr<ClientApi>>;
  std::mutex mu_;
  ClientQueue clients_;
};

class TagPool : public Pool {
protected:
  void SetProtocol(ClientApi* c) override {
	c->SetProtocol("tag", "");
  }
};

// We need separate pools for "normal" clients and "tagged" clients since
// the tag protocol must be set before client.Init is called, and can't be
// changed later without re-initializing the connection.
static Pool defaultPool;
static TagPool tagPool;

// Declare prototypes for exported Go functions.
extern "C" {
  void gop4apiHandleError(int cbid, char* err, int len);
  void gop4apiOutputBinary(int cbid, char* data, int len);
  void gop4apiOutputText(int cbid, char* data, int len);
  void gop4apiOutputInfo(int cbid, char level, char* info);
  void gop4apiOutputStat(int cbid, int count, strview* key, strview* value);
  void gop4apiRetry(int cbid, char* context, char* err, int len);
}

class ClientCb : public ClientUser {
 public:
  ClientCb(int cbid, strview input) :
	ClientUser(0), cbid_(cbid), input_(input) {}
  ~ClientCb() {}

  void HandleError(Error* err) override {
	if (err->Test()) {
	  StrBuf tmp;
	  err->Fmt(&tmp);
	  gop4apiHandleError(cbid_, tmp.Text(), tmp.Length());
	}
  }
  void HandleError(const std::string& msg) {
	gop4apiHandleError(cbid_, const_cast<char*>(msg.c_str()), msg.size());
  }
  void Retry(const std::string& context, Error* err) {
	StrBuf msg;
	if (err) {
	  err->Fmt(&msg);
	}
	gop4apiRetry(cbid_, const_cast<char*>(context.c_str()),
				 msg.Text(), msg.Length());
  }
  void OutputBinary(const char* data, int length) override {
	gop4apiOutputBinary(cbid_, const_cast<char*>(data), length);
  }
  void OutputText(const char* data, int length) override {
	gop4apiOutputText(cbid_, const_cast<char*>(data), length);
  }
  void OutputInfo(char level, const char* data) override {
	gop4apiOutputInfo(cbid_, level - '0', const_cast<char*>(data));
  }
  void OutputStat(StrDict *varList) override {
	std::vector<strview> keys;
	std::vector<strview> values;
	StrRef var, val;

	// Dump out the variables, using the GetVar( x ) interface.
	// Don't display the function, which is only relevant to rpc.
	// Don't display "specFormatted" which is only relevant to us.
	for (int i = 0; varList->GetVar( i, var, val ); i++) {
	  if( var == "func" || var == P4Tag::v_specFormatted ) continue;

	  strview keyview{var.Text(), static_cast<int>(var.Length())};
	  keys.push_back(keyview);
	  strview valview{val.Text(), static_cast<int>(val.Length())};
	  values.push_back(valview);
	}
	gop4apiOutputStat(cbid_, keys.size(), keys.data(), values.data());
  }
  void InputData(StrBuf* buf, Error* e) override {
	if (input_.len > 0) {
	  buf->Append(input_.p, input_.len);
	  buf->Terminate();
	}	
  }

private:
  int cbid_;
  strview input_;
};

extern "C" {
  int p4runcb(strview cmd, strview user, strview passwd, strview input,
			  strview joined, int argc, void* argv, int cbid, bool tag) {
	ClientCb cb(cbid, input);
	std::string cmdstr(cmd.p, cmd.len);
	std::string userStr(user.p, user.len);
	std::string passwdStr(passwd.p, passwd.len);
	int init_us = 0;
	Pool& pool = tag ? tagPool : defaultPool;
	while (true) {
	  std::string errmsg;
	  bool fresh = false;
	  auto c = pool.Client(&init_us, &errmsg, &fresh);
	  if (!c) {
		cb.HandleError(errmsg);
		return init_us;
	  }
	  std::string origUser;
	  std::string origPasswd;
	  if (!userStr.empty() && !passwdStr.empty()) {
		origUser = c->GetUser().Text();
		origPasswd = c->GetPassword().Text();
		c->SetUser(userStr.c_str());
		c->SetPassword(passwdStr.c_str());
	  }
	  
	  // Set arguments.
	  int* args = reinterpret_cast<int*>(argv);
	  int next = 0;
	  for (int i = 0; i < argc; ++i) {
		c->SetVar(StrRef::Null(), StrRef(&joined.p[next], args[i]));
		next += args[i];
	  }
	  c->Run(cmdstr.c_str(), &cb);

	  if (!c->Dropped()) {
		if (!userStr.empty() && !passwdStr.empty())  {
		  // Restore the original user/password for the connection.
		  c->SetUser(origUser.c_str());
		  c->SetPassword(origPasswd.c_str());
		}
		break;
	  }

	  Error err;
	  c->Final(&err);
	  if (err.Test()) {
		cb.Retry("p4 connection dropped: ", &err);
	  }
	}
	return init_us;
  }
}
