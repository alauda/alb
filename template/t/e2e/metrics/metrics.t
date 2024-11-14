
use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;

log_level("info");
master_process_enabled("on");
no_shuffle();
# workers(4); # test for mutli metrics # currently mock init_worker has no way to sync backend in each worker..
no_root_location();
run_tests();

__DATA__

=== TEST 1: clean-metrics
--- mock_backend: 1880 e2e.metrics.metrics
--- timeout: 9999999
--- lua_test_eval: require("e2e.metrics.metrics").test()
