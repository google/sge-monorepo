# Binary Perculator

Perforce stats cacluator

## OVERVIEW

Perculator calculates statistics about perforce and swarm usage.

## DETAILS

Perculator will examine all submitted perforces changelists by executing a `p4 changes` command. It will iterate through each changelist and use `p4 describe` and `p4 sizes` to obtain further details.

All code reviews and comments counts are retrieved from swarm.

All interactions with these services are performed asynchronously, and goroutines are use to maximise effeciency of these interactions.

## BUILDING

Perculator uses IMGUI for its frontend. Builds need to pass the 'config=windows-gnu' to work:

`bazel build -config=windows-gnu //tools/perculator`

## PUBLISHING

Perculator is published to the bin/windows directory of the monorepo:

`sgeb publish //tools/perculator:perculator_bin`


## INDEX :

### Internals:

*   `perculator.go` : backend logic for stats colleciton
*   `gui.go` : imgui front end
