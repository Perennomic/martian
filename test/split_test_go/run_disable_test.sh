#!/bin/bash
MROPATH=$PWD
PATH=$PWD/../../bin:$PATH
if [ -z "$MROFLAGS" ]; then
    export MROFLAGS="--disable-ui"
fi
mrp disable_pipeline.mro disable_pipeline_test
