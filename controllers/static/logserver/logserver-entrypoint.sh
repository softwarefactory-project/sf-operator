#!/bin/bash

sed -i '/LocationMatch/,/LocationMatch/{s/.*/#&/}' /etc/httpd/conf.d/welcome.conf
/usr/bin/run-httpd