# CI Runner

`cirunner` is the application that take cares of all the tasks required for correct setup for
running tasks on a remote CI machine. It is architected as a main application that delegates the
work to an "internal runner". The flow is that cirunner first takes care of setting up the
environment, which means syncing the monodepot, unshelving a CL (if available) and building the
internal runner for source. Which internal runner gets build depends on the invocation proto
provided by the CI machine.

## Internal Runners

Internal runners are binaries that execute a particular CI task. Presubmit and publishing are
examples of internal runners. They have a proto based interface with cirunner, in which cirunner
passes on an `RunnerInvocation` proto which holds the necessary information for execution.

The `RunnerInvocation` proto is defined in `//sge/build/cicd/cirunner/protos/cirun.proto`.
For simplifying the development of internal runners, there is a `runnertool` package that handles
loading the invocation proto at `//sge/build/cicd/cirunner/runnertool`. You can see an example of
the usage in `//sge/build/cicd/cirunner/presubmit_runner/presubmit_runner.go`.

### Installing a new internal runner

As stated before, internal runners gets build from source every time, so there is no need to vendor
them. There is however a "installing procedure":

1. Write the invocation sub message in `//sge/build/cicd/cirunner/protos/cirun.proto`.
2. Add the target to the `findTargetForInvocation` function in `//sge/build/cicd/cirunner/cirunner.go`.
3. Publish the new cirunner by running `sgeb publish //sge/build/cicd/cirunner:publish`.
4. Have a CI machine invoke the cirunner with the correct text proto. You can see a Jenkins example
   of this at `//sge/build/jenkins/pipelines/presubmit.Jenkinsfile`.

## Credentials

In order to run properly, cirunner requires a certain amount of crendentials. We use Google Cloud
Secret Manager to handle credentials. As some credentials might be also needed by some internal
runners (eg. presubmit_runner sends emails), the list of available credentials being loaded are
in the `runnertool` package at `//sge/build/cicd/runnertool/credentials.go`. Note that not all
credentials need to be present for a successful run (eg. a publishing runner might not need a shadow
jenkins credentials).
