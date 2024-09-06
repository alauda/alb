use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;

my $base = $ENV{'TEST_BASE'};
our $cert = <<_EOC_;
$base/cert/tls.crt $base/cert/tls.key
_EOC_

our $http_config = <<_EOC_;
server {
    listen 1880;
    location / {
       content_by_lua_block {
            require("e2e.cert.cert").as_backend(1880)
      }
    }
}
_EOC_


log_level("info");
workers(4);
master_process_enabled("on");
no_shuffle();
no_root_location();
run_tests();

__DATA__

=== TEST 1: cert test
--- certificate eval: $::cert
--- http_config eval: $::http_config
--- alb_https_port: 8443,9443
--- disable_init_worker
--- init_worker_eval: require("mock_worker_init").init_worker()
--- timeout: 9999999
--- lua_test_eval: require("e2e.cert.cert").test()
