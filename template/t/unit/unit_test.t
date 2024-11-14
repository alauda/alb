use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;


log_level("info");
no_shuffle();
no_root_location();
run_tests();


__DATA__

=== TEST 1: unit tests 
--- http_config eval: ""
--- disable_init_worker
--- lua_test_eval: require("unit.unit_test").test()


