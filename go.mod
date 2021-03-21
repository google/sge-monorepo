module sge-monorepo

go 1.16

require (
	bazel.io v0.0.0-00010101000000-000000000000
	cloud.google.com/go v0.78.0
	cloud.google.com/go/datastore v1.4.0
	cloud.google.com/go/logging v1.2.0
	cloud.google.com/go/spanner v1.15.0
	cloud.google.com/go/storage v1.13.0
	contrib.go.opencensus.io/exporter/stackdriver v0.13.5 // indirect
	github.com/AllenDang/giu v0.0.0-20200817121315-321faa16e79b
	github.com/BurntSushi/toml v0.3.1
	github.com/atotto/clipboard v0.1.4
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.4
	github.com/hashicorp/go-version v1.2.1
	github.com/julvo/htmlgo v0.0.0-20200505154053-2e9f4b95a223
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/mb0/glob v0.0.0-20160210091149-1eb79d2de6c4
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	go.opencensus.io v0.22.5 // indirect
	golang.org/x/net v0.0.0-20210119194325-5f4716e94777 // indirect
	golang.org/x/oauth2 v0.0.0-20210220000619-9bb904979d93
	golang.org/x/sys v0.0.0-20210223095934-7937bea0104d
	golang.org/x/text v0.3.5
	google.golang.org/api v0.40.0
	google.golang.org/genproto v0.0.0-20210222212404-3e1e516060db
	google.golang.org/grpc v1.35.0
	google.golang.org/protobuf v1.25.0
	sge-monorepo/build/builders/unity_builder/protos/unity_builderpb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/builders/wwise_builder/protos/soundbankpb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/cicd/cicdfile/protos/cicdfilepb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/cicd/cirunner/protos/cirunnerpb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/cicd/cirunner/runners/cron_runner/protos/cronpb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/cicd/cirunner/runners/postsubmit_runner/protos/postsubmitpb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/cicd/cirunner/runners/publish_runner/protos/publishpb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/cicd/cirunner/runners/unit_runner/protos/unit_runnerpb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/cicd/presubmit/check/protos/checkpb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/cicd/presubmit/protos/presubmitpb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/cicd/sgeb/protos/buildpb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/cicd/sgeb/protos/sgebpb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/packagemanifest/protos/packagemanifestpb v0.0.0-00010101000000-000000000000
	sge-monorepo/build/publishers/docker_publisher/protos/dockerpushconfigpb v0.0.0-00010101000000-000000000000
	sge-monorepo/tools/bazel2vs/protos/msbuildpb v0.0.0-00010101000000-000000000000
	sge-monorepo/tools/p4_benchmark/protos/benchmarkpb v0.0.0-00010101000000-000000000000 // indirect
	sge-monorepo/tools/vendor_bender/protos/licensepb v0.0.0-00010101000000-000000000000
	sge-monorepo/tools/vendor_bender/protos/manifestpb v0.0.0-00010101000000-000000000000
	sge-monorepo/tools/vendor_bender/protos/metadatapb v0.0.0-00010101000000-000000000000
)

replace (
	bazel.io => ./third_party/bazel.io

	// build
	sge-monorepo/build/builders/unity_builder/protos/unity_builderpb => ./proto-gen/sge-monorepo/build/builders/unity_builder/protos/unity_builderpb
	sge-monorepo/build/builders/wwise_builder/protos/soundbankpb => ./proto-gen/sge-monorepo/build/builders/wwise_builder/protos/soundbankpb
	sge-monorepo/build/cicd/cicdfile/protos/cicdfilepb => ./proto-gen/sge-monorepo/build/cicd/cicdfile/protos/cicdfilepb
	sge-monorepo/build/cicd/cirunner/protos/cirunnerpb => ./proto-gen/sge-monorepo/build/cicd/cirunner/protos/cirunnerpb
	sge-monorepo/build/cicd/cirunner/runners/cron_runner/protos/cronpb => ./proto-gen/sge-monorepo/build/cicd/cirunner/runners/cron_runner/protos/cronpb
	sge-monorepo/build/cicd/cirunner/runners/postsubmit_runner/protos/postsubmitpb => ./proto-gen/sge-monorepo/build/cicd/cirunner/runners/postsubmit_runner/protos/postsubmitpb
	sge-monorepo/build/cicd/cirunner/runners/publish_runner/protos/publishpb => ./proto-gen/sge-monorepo/build/cicd/cirunner/runners/publish_runner/protos/publishpb
	sge-monorepo/build/cicd/cirunner/runners/unit_runner/protos/unit_runnerpb => ./proto-gen/sge-monorepo/build/cicd/cirunner/runners/unit_runner/protos/unit_runnerpb
	sge-monorepo/build/cicd/presubmit/check/protos/checkpb => ./proto-gen/sge-monorepo/build/cicd/presubmit/check/protos/checkpb
	sge-monorepo/build/cicd/presubmit/protos/presubmitpb => ./proto-gen/sge-monorepo/build/cicd/presubmit/protos/presubmitpb
	sge-monorepo/build/cicd/sgeb/protos/buildpb => ./proto-gen/sge-monorepo/build/cicd/sgeb/protos/buildpb
	sge-monorepo/build/cicd/sgeb/protos/sgebpb => ./proto-gen/sge-monorepo/build/cicd/sgeb/protos/sgebpb
	sge-monorepo/build/packagemanifest/protos/packagemanifestpb => ./proto-gen/sge-monorepo/build/packagemanifest/protos/packagemanifestpb
	sge-monorepo/build/publishers/docker_publisher/protos/dockerpushconfigpb => ./proto-gen/sge-monorepo/build/publishers/docker_publisher/protos/dockerpushconfigpb

	// tools
	sge-monorepo/tools/bazel2vs/protos/msbuildpb => ./proto-gen/sge-monorepo/tools/bazel2vs/protos/msbuildpb
	sge-monorepo/tools/p4_benchmark/protos/benchmarkpb => ./proto-gen/sge-monorepo/tools/p4_benchmark/protos/benchmarkpb
	sge-monorepo/tools/vendor_bender/protos/licensepb => ./proto-gen/sge-monorepo/tools/vendor_bender/protos/licensepb
	sge-monorepo/tools/vendor_bender/protos/manifestpb => ./proto-gen/sge-monorepo/tools/vendor_bender/protos/manifestpb
	sge-monorepo/tools/vendor_bender/protos/metadatapb => ./proto-gen/sge-monorepo/tools/vendor_bender/protos/metadatapb
)
