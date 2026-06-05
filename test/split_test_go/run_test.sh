#!/bin/bash
MROPATH=$PWD
PATH=$PWD/../../bin:$PATH
if [ -z "$MROFLAGS" ]; then
    export MROFLAGS="--disable-ui"
fi
mrp --overrides=overrides.json pipeline.mro pipeline_test
