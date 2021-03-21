## OVERVIEW

Perforce allows users to perform search across our entire perforce server.
Users do not need to sync files to search, the search is done entirely server side

## BUILDING

Poogle uses IMGUI for its frontend. Builds need to pass the 'config=windows-gnu' to work:

bazel build -config=windows-gnu //tools/poogle

## PUBLISHING

Poogle is published to the bin/windows directory of the monorepo:

sgeb publish //tools/poogle:poogle.bin

## INDEX :

### Internals:

*   `poogle.go` : logic and imgui front end
