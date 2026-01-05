#!/bin/bash
MROPATH=$PWD/../monitor_test
if [ -z "$MROFLAGS" ]; then
    export MROFLAGS="--disable-ui"
fi
PATH=../../bin:$PATH
# must fail on the first try
mrp --monitor ../monitor_test/pipeline.mro pipeline_test_manual_restart && exit 1
# should increase memory and succeed on retry
mrp --monitor --auto-adjust-memory ../monitor_test/pipeline.mro pipeline_test_manual_restart
