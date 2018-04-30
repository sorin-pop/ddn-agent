#!/bin/bash

if [[ $# -ne 1 ]]; then
    echo 'Please specify the version. Should be major.minor.patch (e.g. 3.1.10).'
else
    version=$1
    # rootloc is the base root of the repository
    rootloc=`pwd`/../../

    echo "building binary of server.."
    cd $rootloc/agent
    GOOS=linux go build -ldflags "-X main.version=`date -u +%Y%m%d.%H%M%S`"

    cd $rootloc/dist/agent

    for d in */; do
        dir=${d%/}

        echo "Creating $dir"

        cp -r $rootloc/agent/agent $rootloc/agent/sql $dir

        docker build -t agent-$dir:$version -t agent-$dir:latest $dir

        docker tag agent-$dir:$version djavorszky/ddn-agent-$dir:$version
        docker push djavorszky/ddn-agent-$dir:$version
        docker tag agent-$dir:latest djavorszky/ddn-agent-$dir:latest
        docker push djavorszky/ddn-agent-$dir:latest

        rm -rf $dir/agent $dir/sql

        echo "Created $dir"
    done
fi