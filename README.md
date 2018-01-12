# rdl-gen-json-schema
A plugin for the [RDL](https://github.com/ardielle/ardielle-tools) command line tool
to generate JSON schemas from RDL type definitions.

This implementation is a quick hack, is not complete at all, just enough to generate a few schemas I needed.

## Installing

	go get github.com/boynton/rdl-gen-json-schema

And then make sure the $GOPATH/bin/rdl-gen-json-schema executable is in your path. 

## Using

	rdl generate json-schema your.rdl

This will output a json-schema file to stdout. Or, to place the output in a current directory, for example:

	rdl generate -o . json-schema your.rdl

This will create a file called your.json in the specified directory.


