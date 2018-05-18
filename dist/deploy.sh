#!/bin/bash

if [[ $1 == "--push" ]]; then
    push="true"
    shift 1
fi

if [[ $# -ne 1 ]]; then
    echo 'Please specify the version. Should be major.minor.patch (e.g. 3.1.10).'
else
    version=$1
    # rootloc is the base root of the repository
    rootloc=`pwd`/../

    echo "building binary of agent.."
    cd $rootloc
    GOOS=linux go build -ldflags "-X main.version=`date -u +%Y%m%d.%H%M%S`"

    cd $rootloc/dist

    for d in */; do
        dir=${d%/}

        echo "Building $dir"

        cp -r $rootloc/ddn-agent $rootloc/sql $dir

        docker build -t djavorszky/ddn-agent-$dir:latest -t djavorszky/ddn-agent-$dir:$version $dir

        if [[ $push == "true" ]]; then
            docker push djavorszky/ddn-agent-$dir
        fi

        rm -rf $dir/ddn-agent $dir/sql

        echo "Built $dir"
    done
fi