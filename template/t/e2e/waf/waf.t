use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;

my $ALB_BASE = $ENV{'TEST_BASE'};
our $tt = t::Alauda::get_test_name(__FILE__);
our $loc = <<"END_TXT";
- name: modsecurity_redirect_p1
  locationRaw: |
       modsecurity on;
       modsecurity_rules '
         SecRuleEngine On
         SecDebugLog $ALB_BASE/servroot/logs/modsec_debug.log
         SecRule ARGS:testparam "\@contains redirect" "id:1234,status:302,redirect:http://a.com"
         SecDebugLogLevel 9
         SecRuleRemoveById 10
       ';
- name: modsecurity_p1
  locationRaw: |
       modsecurity on;
       modsecurity_rules '
         SecRuleEngine On
         SecDebugLog $ALB_BASE/servroot/logs/modsec_debug.log
         SecRule ARGS:testparam "\@contains test" "id:1234,deny,log"
         SecDebugLogLevel 9
         SecRuleRemoveById 10
       ';
- name: modsecurity_p2
  locationRaw: |
       modsecurity on;
       modsecurity_rules '
         SecRuleEngine On
         SecDebugLog $ALB_BASE/servroot/logs/modsec_debug.log
         SecRule REQUEST_HEADERS:Content-Type "^application/json" "id:200001,phase:1,t:none,t:lowercase,pass,nolog,ctl:requestBodyProcessor=JSON"
         SecRule ARGS:json.a "\@contains b" "id:1001,phase:2,deny,log"
         SecDebugLogLevel 9
         SecRuleRemoveById 10
       ';
- name: modsecurity_p3
  locationRaw: |
       modsecurity on;
       modsecurity_rules '
         SecRuleEngine On
         SecDebugLog $ALB_BASE/servroot/logs/modsec_debug.log
         SecRule RESPONSE_HEADERS:X-RET "\@contains b" "id:9001,phase:3,deny,log"
         SecDebugLogLevel 9
         SecRuleRemoveById 10
       ';
- name: modsecurity_p4
  locationRaw: |
       modsecurity on;
       modsecurity_rules '
         SecRuleEngine On
         SecResponseBodyAccess On 
         SecDebugLog $ALB_BASE/servroot/logs/modsec_debug.log
         SecRule RESPONSE_BODY "\@contains b" "id:9001,phase:4,deny,log"
         SecDebugLogLevel 9
         SecRuleRemoveById 10
           ';
- name: modsecurity_rule
  locationRaw: |
       modsecurity on;
       modsecurity_rules '
        SecRuleEngine On
       ';
       modsecurity_rules_file /etc/nginx/owasp-modsecurity-crs/nginx-modsecurity.conf;
END_TXT
log_level("info");
workers(1);
no_shuffle();
no_root_location();
run_tests();
__DATA__

=== TEST 1: waf
--- custom_location_raw eval: $::loc
--- mock_backend eval: "1880 $::tt"
--- init_worker_eval: require("mock_worker_init").init_worker()
--- lua_test_eval eval: "require('$::tt').test()"
