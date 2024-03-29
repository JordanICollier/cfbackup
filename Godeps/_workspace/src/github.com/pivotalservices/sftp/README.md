sftp
----

The `sftp` package provides support for file system operations on remote ssh servers using the SFTP subsystem.

[![Build Status](https://drone.io/github.com/pivotalservices/sftp/status.png)](https://drone.io/github.com/pivotalservices/sftp/latest)

usage and examples
------------------

See [godoc.org/github.com/pivotalservices/sftp](http://godoc.org/github.com/pivotalservices/sftp) for examples and usage.

The basic operation of the package mirrors the facilities of the [os](http://golang.org/pkg/os) package. 

The Walker interface for directory traversal is heavily inspired by Keith Rarick's [fs](http://godoc.org/github.com/kr/fs) package.

roadmap
-------

 * Currently all traffic with the server is serialized, this can be improved by allowing overlapping requests/responses.
 * There is way too much duplication in the Client methods. If there was an unmarshal(interface{}) method this would reduce a heap of the duplication.
 * Implement integration tests by talking directly to a real opensftp-server process. This shouldn't be too difficult to implement with a small refactoring to the sftp.NewClient method. These tests should be gated on an -sftp.integration test flag. _in progress_

contributing
------------

Features, Issues, and Pull Requests are always welcome.
