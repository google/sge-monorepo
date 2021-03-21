# Environment

These are a set of components and redistributables needed for a system to correctly run SG&E
software. The install program will install the dependencies and write a marker file with the
newly installed version. This will permit agents such as worker to avoid reinstalling dependencies
they already have present in the system. Normal callers will always write the installation so that
they can always ensure they are at the latest version.

All the the install files are within the `data` directory.

