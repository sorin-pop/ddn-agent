#!/bin/bash

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

        echo "Creating $dir"

        cp -r $rootloc/ddn-agent $rootloc/sql $dir

        docker build -t agent-$dir:$version -t agent-$dir:latest $dir

        docker tag agent-$dir:$version djavorszky/ddn-agent-$dir:$version
        docker push djavorszky/ddn-agent-$dir:$version
        docker tag agent-$dir:latest djavorszky/ddn-agent-$dir:latest
        docker push djavorszky/ddn-agent-$dir:latest

        rm -rf $dir/ddn-agent $dir/sql

        echo "Created $dir"
    done
fi