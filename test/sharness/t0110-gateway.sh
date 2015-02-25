#!/bin/sh
#
# Copyright (c) 2015 Matt Bell
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test HTTP Gateway"

. lib/test-lib.sh

test_init_ipfs
test_config_ipfs_gateway_readonly "/ip4/0.0.0.0/tcp/5002"
test_launch_ipfs_daemon

webui_hash="QmXdu7HWdV6CUaUabd9q2ZeA4iHZLVyDRj3Gi4dsJsWjbr"

# TODO check both 5001 and 5002.
# 5001 should have a readable gateway (part of the API)
# 5002 should have a readable gateway (using ipfs config Addresses.Gateway)
# but ideally we should only write the tests once. so maybe we need to
# define a function to test a gateway, and do so for each port.
# for now we check 5001 here as 5002 will be checked in gateway-writable.

test_expect_success "GET IPFS path succeeds" '
  echo "Hello Worlds!" > expected &&
  HASH=`ipfs add -q expected` &&
  wget "http://127.0.0.1:5002/ipfs/$HASH" -O actual
'

test_expect_success "GET IPFS path output looks good" '
  test_cmp expected actual &&
  rm actual
'

test_expect_success "GET IPFS directory path succeeds" '
  mkdir dir &&
  echo "12345" > dir/test &&
  HASH2=`ipfs add -r -q dir | tail -n 1` &&
  wget "http://127.0.0.1:5002/ipfs/$HASH2"
'

test_expect_success "GET IPFS directory file succeeds" '
  wget "http://127.0.0.1:5002/ipfs/$HASH2/test" -O actual
'

test_expect_success "GET IPFS directory file output looks good" '
  test_cmp dir/test actual
'

test_expect_failure "GET IPNS path succeeds" '
  ipfs name publish "$HASH" &&
  NAME=`ipfs config Identity.PeerID` &&
  wget "http://127.0.0.1:5002/ipns/$NAME" -O actual
'

test_expect_failure "GET IPNS path output looks good" '
  test_cmp expected actual
'

test_expect_success "GET invalid IPFS path errors" '
  test_must_fail wget http://127.0.0.1:5002/ipfs/12345
'

test_expect_success "GET invalid path errors" '
  test_must_fail wget http://127.0.0.1:5002/12345
'


test_expect_success "GET webui forwards" '
  curl -I http://127.0.0.1:5001/webui | head -c 18 > actual1
  curl -I http://127.0.0.1:5001/webui/ | head -c 18 > actual2
  echo "HTTP/1.1 302 Found" | head -c 18 > expected
  echo "HTTP/1.1 301 Moved Permanently" | head -c 18 > also_ok
  cat actual1
  cat actual2
  cat expected
  cat also_ok
  (test_cmp expected actual1 || test_cmp actual1 also_ok)&&
  (test_cmp expected actual2 || test_cmp actual2 also_ok)&&
  rm expected &&
  rm also_ok &&
  rm actual1 &&
  rm actual2
'

#TODO make following test work
#test_expect_success "GET webui loads" '
#  curl http://127.0.0.1:5001/ipfs/$webui_hash | head -n 1 > actual1
#
#  rm expected &&
#  rm actual1 &&
#  rm actual2
#'

test_kill_ipfs_daemon

test_done
