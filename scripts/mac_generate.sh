#!/bin/bash

echo "52:`(openssl rand -hex 5 | sed 's/\(..\)\(..\)\(..\)\(..\)\(..\)/\1:\2:\3:\4:\5/')`"