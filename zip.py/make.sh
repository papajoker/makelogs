#!/usr/bin/env bash

clear
[ -f ./makelogs ] && rm -v ./makelogs
echo 'gen: "makelog" binary'
python -m zipapp src --python="/usr/bin/env python" --output="makelogs" --compress
unzip -l makelogs
ls -l makelogs
if [[ -n "$1" ]]; then
    ./makelogs "$@" 
    #ls -l /tmp/makelogs/yaml/*.*
    echo
    tree /tmp/makelogs/
fi
