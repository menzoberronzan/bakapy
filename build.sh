#!/bin/sh

export GOPATH="`pwd`/vendor:`pwd`"

go install bakapy-scheduler && go install bakapy-show-meta && go install bakapy-run-job && go install bakapy-storage && go install bakapy-metaman
