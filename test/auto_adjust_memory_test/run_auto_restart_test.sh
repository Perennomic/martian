#!/bin/bash
MROPATH=$PWD/../monitor_test
if [ -z "$MROFLAGS" ]; then
    export MROFLAGS="--disable-ui"
fi
PATH=../../bin:$PATH
mrp --monitor --auto-adjust-memory ../monitor_test/pipeline.mro pipeline_test_auto_restart
