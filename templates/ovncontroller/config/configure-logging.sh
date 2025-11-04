#!/bin/bash
#
# Copyright 2024 Red Hat Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
# License for the specific language governing permissions and limitations
# under the License.

# This script configures ovn-encap-tos setting in OVS external-ids
# It is only used when ovn-encap-tos is explicitly set to a non-default value or
# when OVNLogLevel or OVSLogLevel is set to a non-default value.

source $(dirname $0)/../container-scripts/functions

OVSLogLevel={{.OVSLogLevel}}
OVNLogLevel={{.OVNLogLevel}}

function configure_logging {
    /usr/bin/ovn-appctl vlog/set ${OVNLogLevel}
}

# Set the log level for ovn-controller
wait_for_ovn_controller
configure_logging
