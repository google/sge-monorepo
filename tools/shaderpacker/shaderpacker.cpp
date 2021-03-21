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

#include <iostream>
#include <regex>
#include <fstream>
#include <sstream>
#include <vector>

#include <atlbase.h>
#include <direct.h>

/*******************************************************************************************/
// TODO_RW: replace this with #include <filesystem> once we change language standard to c++17
#define _SILENCE_EXPERIMENTAL_FILESYSTEM_DEPRECATION_WARNING
#include <experimental/filesystem>
/*******************************************************************************************/

#include "dxcapi.h"


// uncomment to disable assert()
// #define NDEBUG
#include <cassert>
#include <stdint.h>
#include "tools/shaderpacker/shader_header_generated.h"
using namespace std;
using namespace Render::Shader;


/***************************************************************************************************************/
// Validation
/***************************************************************************************************************/

struct SShaderEntry
{
	string      name;
	uint8_t* shader = nullptr;
	size_t      shader_size = 0;
	ShaderType type;

	~SShaderEntry() { delete[] shader; }
};

struct RenderShader
{
	std::string name;
	Render::Shader::ShaderType type;
	uint8_t* shader = nullptr;
	size_t shader_size = 0;

	~RenderShader() { delete[] shader; }
};

struct RenderEffect
{
	std::string name;
	uint32_t shader_count = 0;
	std::vector<RenderShader> shaders;
};

void UnPackShaders(char* buffer, RenderEffect& render_effect)
{
	auto shader_package = GetShaderPackage(buffer);
	auto shaders = shader_package->shaders();

	render_effect.name = string(shader_package->name()->c_str());
	render_effect.shader_count = shaders->size();
	render_effect.shaders.resize(render_effect.shader_count);

	for (uint32_t i = 0; i < render_effect.shader_count; ++i)
	{
		auto shader = shaders->Get(i);
		render_effect.shaders[i].name = string(shader->entry_point()->c_str());
		render_effect.shaders[i].type = shader->shader_type();
		const size_t shader_size = shader->data()->size();
		render_effect.shaders[i].shader_size = shader_size;
		render_effect.shaders[i].shader = new uint8_t[shader_size];
		memcpy(render_effect.shaders[i].shader, shader->data()->data(), shader_size);
	}
}

void Validate(const string& file_name, const vector<SShaderEntry>& shader_entries)
{
	// validation
	/*************************************************************************/
	// TODO_RW: remove experimental:: once we change language standard to c++17
	size_t file_size = experimental::filesystem::file_size(file_name);
	/*************************************************************************/
	ifstream infile(file_name, ifstream::binary);
	char* buffer = new char[file_size];
	infile.read(buffer, file_size);
	RenderEffect render_effect;
	UnPackShaders(buffer, render_effect);

	for (size_t i = 0; i < render_effect.shader_count; ++i)
	{
		const auto& shaders = render_effect.shaders;
		assert(shaders[i].shader_size == shader_entries[i].shader_size);
		assert(memcmp(shaders[i].shader, shader_entries[i].shader, shader_entries[i].shader_size) == 0);
	}
	delete[] buffer;
}

/***************************************************************************************************************/

namespace shader_packer_util
{
    bool CompareString(const string& str1, const string& str2)
    {
        return str1.size() == str2.size() && equal(str1.begin(), str1.end(), str2.begin(), [](auto a, auto b) {return tolower(a) == tolower(b); });
    }

    ShaderType GetShaderType(const string& inType)
    {
        if (CompareString(inType, "vertex")) return ShaderType::ShaderType_Vertex;
        if (CompareString(inType, "pixel")) return ShaderType::ShaderType_Pixel;
        if (CompareString(inType, "geometry")) return ShaderType::ShaderType_Geometry;
        if (CompareString(inType, "hull")) return ShaderType::ShaderType_Hull;
        if (CompareString(inType, "domain")) return ShaderType::ShaderType_Domain;
        if (CompareString(inType, "compute")) return ShaderType::ShaderType_Compute;

        assert(false && "Unsupported shader type!!!");
        return ShaderType::ShaderType_Vertex;
    }

    string GetTargetProfile(ShaderType type)
    {
        switch (type)
        {
        case ShaderType::ShaderType_Vertex: return "vs_6_0";
        case ShaderType::ShaderType_Pixel: return "ps_6_0";
        case ShaderType::ShaderType_Geometry: return "gs_6_0";
        case ShaderType::ShaderType_Hull: return "hs_6_0";
        case ShaderType::ShaderType_Domain: return "ds_6_0";
        case ShaderType::ShaderType_Compute: return "cs_6_0";
        default: assert(false && "Unsupported shader type!!!"); return "Unknown";
        }
    }
}

void PackShaders(const vector<SShaderEntry>& shader_entries, const string& shader_name, const string& output_path, const string& output_file_name)
{
	// reserve space for headers
	const uint32_t entry_count = (uint32_t)shader_entries.size();
    flatbuffers::FlatBufferBuilder builder(1024);
    vector<flatbuffers::Offset<ShaderHeader>> shader_vec;
    shader_vec.resize(entry_count);
    for (uint32_t i = 0; i < entry_count; ++i)
    {
        auto entry_name = builder.CreateString(shader_entries[i].name);
        auto shader_data = builder.CreateVector(shader_entries[i].shader, shader_entries[i].shader_size);

		ShaderHeaderBuilder sh_builder(builder);
        sh_builder.add_data(shader_data);
        sh_builder.add_entry_point(entry_name);
        sh_builder.add_shader_type(shader_entries[i].type);
        shader_vec[i] = sh_builder.Finish();
    }

	auto package_name = builder.CreateString(shader_name);
    auto shaders = builder.CreateVector(shader_vec);
	ShaderPackageBuilder pkg_builder(builder);
    pkg_builder.add_shaders(shaders);
    pkg_builder.add_name(package_name);
    auto package = pkg_builder.Finish();

    builder.Finish(package);
     
    // save on to disk
    /*************************************************************************/
    // TODO_RW: remove experimental:: once we change language standard to c++17
    experimental::filesystem::create_directories(output_path);
    /*************************************************************************/
    ofstream outfile(output_file_name, ofstream::binary);
    assert(outfile && "Cannot create file stream!");
    outfile.write(reinterpret_cast<const char*>(builder.GetBufferPointer()), builder.GetSize());
    outfile.close();
}

void Compile(const string& shader, SShaderEntry& entry, const string& target_profile, const string& shader_path_file_name)
{
    string entry_name = entry.name;
    wstring w_entry_point = wstring(entry_name.begin(), entry_name.end());
    wstring w_target_profile = wstring(target_profile.begin(), target_profile.end());
    wstring w_shader_path_file_name = wstring(shader_path_file_name.begin(), shader_path_file_name.end());
    CComPtr<IDxcLibrary> library;
    HRESULT hr = DxcCreateInstance(CLSID_DxcLibrary, IID_PPV_ARGS(&library));
    if(FAILED(hr)) 
    {
        assert(false && "Fail to create dxc instance!");
        return;
    }

    CComPtr<IDxcCompiler> compiler;
    hr = DxcCreateInstance(CLSID_DxcCompiler, IID_PPV_ARGS(&compiler));
    if (FAILED(hr))
    {
        assert(false && "Fail to create dxc instance!");
        return;
    }

    uint32_t codePage = CP_UTF8;
    CComPtr<IDxcBlobEncoding> sourceBlob;
    const char* shader_ptr = shader.c_str();
    hr = library->CreateBlobWithEncodingFromPinned(shader_ptr, (unsigned int)shader.size(), CP_UTF8, &sourceBlob);
    if(FAILED(hr)) 
    {
        assert(false && "Fail to create shader blob!");
        return;
    }
    const wchar_t* args[] =
    {
        L"-spirv",			// Generates SPIR-V code
        L"-fspv-reflect",	// Emits additional SPIR-V instructions to aid reflection
       // L"-Zpr",			// Packs matrices in row-major order by default
    };
    CComPtr<IDxcOperationResult> result;
    hr = compiler->Compile(
        sourceBlob, // pSource
        w_shader_path_file_name.c_str(), // pSourceName
        w_entry_point.c_str(), // pEntryPoint
        w_target_profile.c_str(), // pTargetProfile
        args, sizeof(args) / sizeof(args[0]), // pArguments, argCount
        nullptr, 0, // pDefines, defineCount
        nullptr, // pIncludeHandler
        &result); // ppResult
    if (SUCCEEDED(hr))
        result->GetStatus(&hr);
    if (FAILED(hr))
    {
        if (result)
        {
            CComPtr<IDxcBlobEncoding> errorsBlob;
            hr = result->GetErrorBuffer(&errorsBlob);
            if (SUCCEEDED(hr) && errorsBlob)
            {
                const char* error_msg = (const char*)errorsBlob->GetBufferPointer();
                cout << "Shader Compile Failed: " << error_msg << endl;
            }
        }
        // Handle compilation error...
    }
    CComPtr<IDxcBlob> code;
    result->GetResult(&code);

    entry.shader = new uint8_t[code->GetBufferSize()];
    entry.shader_size = code->GetBufferSize();
    memcpy(entry.shader, code->GetBufferPointer(), code->GetBufferSize());
}

int main(int argc, char** argv)
{
    if (argc != 4)
    {
        // using "-T" to be consistent with dxc cmd
        cout << "cmd: shaderpacker -T [outputfilename] [inputfilename]" << endl;
        return 1;
    }

    const string shader_path_file_name(argv[3]);
    const string output_path_file_name(argv[2]);
    ifstream shader_file(shader_path_file_name);
    if (shader_file.is_open())
    {
        // read shader with meta data
        stringstream s;
        s << shader_file.rdbuf();
        vector<SShaderEntry> shader_entries;
        string shader = s.str();
        shader_file.close();

        regex expr("@\\w+\\(.*?\\)");
        sregex_iterator next(shader.begin(), shader.end(), expr);
        sregex_iterator end;
        // collect shaders
        while (next != end)
        {
            smatch sh_match = *next;
            const auto& nts = sh_match.str();
            cout << nts << "\n";
            {
                regex nametype("\\w+");
                sregex_iterator nt_next(nts.begin(), nts.end(), nametype);
                string type = (*nt_next).str();
                cout << "type: " << type << "\n";
                if (shader_packer_util::CompareString(type, "shader"))
                {
                    string sh_name = (*(++nt_next)).str();
                    cout << "shader name: " << sh_name << "\n";
                    string sh_type = (*(++nt_next)).str();
                    cout << "shader type: " << sh_type << "\n";

                    shader_entries.push_back({ sh_name, nullptr, 0, shader_packer_util::GetShaderType(sh_type) });
                }
                else
                {
                    cout << "Unsupported syntax!!!" << endl;
                    assert(false && "Unsupported syntax!!!");
                }
            }
            next++;
        }

        if (shader_entries.empty())
        {
            cout << "Cannot find any entry point!!!" << endl;
            cout << "Make sure to declare all entrypoints by adding @shader([entrypointname], [shadertype]) in the shader file." << endl;
            return 1;
        }

        // remove shader entry info
        string newshader = regex_replace(shader, expr, "");

        // TODO_RW: multi-thread this?
        for (auto& e : shader_entries)
        {
            Compile(newshader, e, shader_packer_util::GetTargetProfile(e.type), shader_path_file_name);
        }

        // get shader file name
        size_t lastdot = shader_path_file_name.find_last_of(".");
        string shader_name_no_ext = (lastdot == string::npos) ? shader_path_file_name : shader_path_file_name.substr(0, lastdot);
        size_t lastslash = shader_path_file_name.find_last_of("\\/");
        string shader_path = (lastslash == string::npos) ? shader_path_file_name : shader_path_file_name.substr(0, lastslash);
        string shader_name = (lastslash == string::npos) ? shader_name_no_ext : shader_name_no_ext.substr(lastslash + 1, shader_name_no_ext.size());


        size_t output_lastslash = output_path_file_name.find_last_of("\\/");
        string output_path = (output_lastslash == string::npos) ? output_path_file_name : output_path_file_name.substr(0, output_lastslash);

        PackShaders(shader_entries, shader_name, output_path, output_path_file_name);

        // validation
        //Validate(output_path_file_name, shader_entries);
    }
    else
    {
        assert(false && "Cannot find the shader file!!!");
    }

    return 0;
}

