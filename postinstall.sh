#!/bin/sh



if [ -f /usr/local/bin/netcheck ]; then
    printf "Setting up \033[32munprivileged raw socket access\033[0m on /usr/local/bin/netcheck: "
    ERROR="$(sudo setcap cap_net_raw=+ep /usr/local/bin/netcheck 2>&1 > /dev/null)"
    if [ $? -eq 0 ]; then
        echo "\033[32msuccess\033[0m!\n"
    else
        echo "\033[31mfailed\033[0m! ($ERROR)\n"
    fi    
fi
