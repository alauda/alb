
use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;

my $ALB_BASE = $ENV{'TEST_BASE'};

our $tt = t::Alauda::get_test_name(__FILE__);

log_level("info");
master_process_enabled("on");
no_shuffle();
no_root_location();
run_tests();

__DATA__

=== TEST 1: timeout
--- mock_backend eval: "1880 $::tt"
--- lua_test_eval eval: "require('$::tt').test()"

=== TEST 2: timeout tcp
--- mock_backend eval: "1880 $::tt"
--- lua_test_stream_mode: "true"
--- lua_test_eval eval: "require('$::tt').test()"
