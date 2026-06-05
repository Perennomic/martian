#!/bin/bash
MROPATH=$PWD
if [ -z "$MROFLAGS" ]; then
    export MROFLAGS="--disable-ui --localmem=3 --limit-loadavg"
fi
PATH=../../bin:$PATH
mrp pipeline.mro loadavg_pipeline_test
