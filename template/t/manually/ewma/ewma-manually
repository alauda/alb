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
worker_connections(5120000);
no_root_location();
run_tests();

__DATA__

=== TEST 1: ewma
--- mock_backend eval: "1880 $::tt"
--- lua_test_eval eval: "require('$::tt').test()"
