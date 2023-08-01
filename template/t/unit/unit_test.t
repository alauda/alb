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
--- policy eval: ""
--- http_config eval: ""
--- lua_test
    require("unit.unit_test").test()

