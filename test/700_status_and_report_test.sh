#! /bin/bash

. ./config.sh

UNIVERSE=10.2.0.0/16

start_suite "weave status/report"

weave_on $HOST1 launch --ipalloc-range $UNIVERSE --name 8a:3e:3e:3e:3e:3e --nickname nicknamington

check() {
    assert "weave_on $HOST1 status | grep -oP '(?<= $1: ).*'" "$3"
    assert "weave_on $HOST1 report -f '$2'" "$3"
}

check "Name"          "{{.Router.Name}}({{.Router.NickName}})" "8a:3e:3e:3e:3e:3e(nicknamington)"
check "Peers"         "{{len .Router.Peers}}"                  "1"
check "DefaultSubnet" "{{.IPAM.DefaultSubnet}}"                $UNIVERSE
check "Domain"        "{{.DNS.Domain}}"                        "weave.local."

assert_raises "weave_on $HOST1 status peers | grep nicknamington"
weave_on $HOST1 connect 10.2.2.1
assert "weave_on $HOST1 status targets" "10.2.2.1"
assert "weave_on $HOST1 status connections | tr -s ' ' | cut -d ' ' -f 2" "10.2.2.1:6783"
start_container $HOST1 --name test
assert "weave_on $HOST1 status dns         | tr -s ' ' | cut -d ' ' -f 1" "test"
assert_raises "weave_on $HOST1 report | grep nicknamington"

end_suite
