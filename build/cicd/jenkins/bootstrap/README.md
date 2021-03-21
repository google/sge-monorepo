# Jenkins Perforce Bootstrap

These are a set of scripts that are meant to bootstrap the Perforce environment and within a
pipeline. The objective is to make pipelines the least intelligent as and push the logic into the
actual tooling being checked out.

For that objective, we need to be able to boostrap the Perforce process. After this process, tooling
should be able to find and authenticated and valid client and have "p4" present in PATH.

## Obtain the Perforce components.

IMPORTANT: This scripts expect `p4.exe` to be present in PATH in order to drive all of this.
Normally this would be done by storing these scripts with p4.exe next to it.

These are all the things needed for having a valid perforce installation in a Jenkins pipeline.
These are all the batch scripts in this directory:

- **bootstrap.bat**: Creates a new P4 client based on a given template and syncs the cirunner.
- **set_perforce_env.bat**: Will change the CWD to the perforce checkout and set p4 to PATH.

The following files are considered secrets and are expected to be present.

- **p4config.txt**: Simple P4CONFIG that sets P4USER, P4PORT, P4CHARSET and other configurations.
                    Can be found in go/valentine under `[sge-ci][*env*][jenkins] p4config.txt`.
- **p4tickets.txt**: Ticket for the service account client. This ticket doesn't expire.
                     Can be found in go/valentine under `[sge-ci][*env*][jenkins] p4tickets.txt`.

## Distribution

The pipeline needs to have all those elements in order to correctly get the elements. The current
distribution is to have a GCS bucket that holds all the elements and checks them out to
`C:\Artifacts`. From there it can start the bootstrap process.

Future processes should be to use Google Cloud Secrets and drive credentials from there.

## Usage

Once the elements are distributed, pipeline would begin with the following step:

```
mkdir C:\\artifacts
gsutil cp gs://<GCS_BUCKET>/p4/* C:\\Artifacts
call C:\\artifacts\\bootstrap ${env.NODE_NAME}
```

After that, it can easily set the environment (making P4 available on path).

```
call C:\\artifacts\\set_perforce_env
call sge\\build\\cicd\\cirunner\\windows\\cirunner presubmit
```

## Sample Pipeline Example

This is a sample Jenkins pipeline for a presubmit run. This is not considering pass/fail emailing,
but otherwise it's a fairly representative example.

```
pipeline {
  agent { node { label "windows-presubmit" }}
  stages {
    stage('Bootstrap') {
      // There should also be a config text proto written here for cirunner to consume. This
      // is skipped in the sake of brevity.
      steps {
        bat """
          mkdir c:\\artifacts
          call C:\\artifacts\\bootstrap Workspace-${env.NODE_NAME}
        """
      }
    }
    stage('Presubmit') {
      steps {
        bat """
          call c:\\artifacts\\set_perforce_env
          call c:\\artifacts\\cirunner -credentials=<CREDS> -context=<CONTEXT> presubmit
        """
      }
    }
  }
}
```
