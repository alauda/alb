use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';


my $base = $ENV{'TEST_BASE'};
our $cert = <<_EOC_;
$base/cert/tls.crt $base/cert/tls.key
_EOC_
our $http_config = <<_EOC_;
server {
    listen 880;
    location / {
       content_by_lua_block {
            local h, err = ngx.req.get_headers()
            if err ~=nil then
                ngx.say("err: "..tostring(err))
            end
            for k, v in pairs(h) do
                ngx.say("header "..tostring(k).." : "..tostring(v))
            end
                ngx.say("url "..ngx.var.request_uri)
            ngx.say("http client-ip "..ngx.var.remote_addr.." client-port "..ngx.var.remote_port.." server-ip "..ngx.var.server_addr.." server-port "..ngx.var.server_port)
      }
    }
}
server {
    listen       8443 ssl;
    server_name  _;

    ssl_certificate     $base/cert/tls.crt;
    ssl_certificate_key $base/cert/tls.key;
    ssl_dhparam $base/share/dhparam.pem;

    ssl_session_timeout  5m;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE:ECDH:AES:HIGH:!NULL:!aNULL:!MD5:!ADH:!RC4;
    ssl_protocols TLSv1 TLSv1.1 TLSv1.2;
    ssl_prefer_server_ciphers on;

    location /ping {
      content_by_lua_block {
          ngx.say("im https")
      }
    }
}
_EOC_


our $policy = <<'_EOC_';
{
  "certificate_map": {
    "443": {
      "key":"-----BEGIN PRIVATE KEY-----\n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\nYUQmiK6nOA0nP4T5x0A6LTbN6AOcxeg=\n-----END PRIVATE KEY-----",
      "cert":"-----BEGIN CERTIFICATE-----\nMIIFDzCCAvegAwIBAgIURfAGhgnCVBovG1GXafPioc7ln/kwDQYJKoZIhvcNAQEL\nBQAwFzEVMBMGA1UEAwwMdGVzdC5hbGIuY29tMB4XDTIxMDkyNjAzMTkxOVoXDTMx\nMDkyNDAzMTkxOVowFzEVMBMGA1UEAwwMdGVzdC5hbGIuY29tMIICIjANBgkqhkiG\n9w0BAQEFAAOCAg8AMIICCgKCAgEAr1AlmjGNbn3pVxonTlur42btuJYxkhC+P3Vt\ntZuVNKR3LxMPlMCCJ1jnJA3y9IVzh1KuJvZQExEC1DXyOhFdQHEweLLic7NSgNEa\nBFI4gEmU56W/StjR10BBiPwqTDLRR3/rVNfEkwYPc103J7QZpBWIzLpxSPdIpnPa\nfLndROjvuXZoQsZPlQwrUQg19DKWQrCsnzNBGD1LqRajOIeY0AUmied8sMXkd0rq\nIRVe1fJbIUIvYCtMHAIS4R1S69mCdy18wdBpYfrpPamOEs3L9gtZxBeEPFLiOz7+\n6jaQI0lXJ5l3+364rOykjdYa7PRpRUPP7GkP6KBBto2x4YGKPS/UuunqL50GWzjx\naM/QCs1RyekDJCR2+4BAE79Ke2HLx6d9dj6M+Kq3vCTIsfFDR6f5kkAKnXJuNTbP\nkoIx+Gg2uwbNiPSGBKdNZ0QWRYZDG3sb+cTyFL9RifawYnncWPVKsYxeCTgc+wcm\nwMh90gzcrrkGwVAIO6gvFQFiBpOFuc0BLgfhPZ0t9BWHC3epEKuKKXwbPZbQzppp\n72wkr/pdmPngBL9gSye0O+4qL3Aoy/T/NbemkwIFXToOm+hX6Nea9QafItWN5LSg\n13fGht87+mciNZ2xTXpcJvcZ6MdtqHH8/+1iaZLleO8gCaLa55NC3ieIghiSLnnz\ntKw4mDECAwEAAaNTMFEwHQYDVR0OBBYEFJWEB7GtSJ1glNCtoJLEnRtbviL2MB8G\nA1UdIwQYMBaAFJWEB7GtSJ1glNCtoJLEnRtbviL2MA8GA1UdEwEB/wQFMAMBAf8w\nDQYJKoZIhvcNAQELBQADggIBAEkeG+Kiar98U9TB4IZpiqo2dw38Zk8fcPJK6wIT\n5104F07DCErYcBp4LXWTJX9iVsdkiiVSE/FmqQjWeX5kACUuM8HkeziQNj++EcTq\nDQtDk9zEBWGDYQH4RQZvIVQlIaieZOArSbhunIrxlGr6fX//ryLO5K4QAay/oqwb\nUXFYfQ0M7VdrsTwLRImQN5KYAJbdR/Nlm5i/fTJps5iSdgpovBAommb1XpVq/aJ5\n3tAqb64vBNigZ7T8V1cCh6oDoCO/xzoucTzF9e14LkTmtzYxBjhplgwSUH6R0cgi\ndiexT2mdBtU79iJ0K5AJFVa1UCR0OE3/FmWEkb4L01XxU5sEyYL6I0JPSXaDdjtL\nv3y2GZY2Iz27qjz/JSZXoyf28rAYE0YHI3nX1wBwDTSoPKnfSc1A/IFsXFfkGB00\nuFNiI5rRff+zBt0XCAEz2Q9aULI5Ho8kjdOqHT/ty6c9RbxnJHv3mRQy0kZsb8QM\nDHTqwEvHE7mwGtd5LD5z6SRQpCfQmoSDuNqUdxMLNIYn45+BZyAlHE6Le21fH/Rb\nCjWQ5fBl7QdBHGB9dpYu8dhdrOlN0xj1QJKJGrVkqOA4nGmD6GThBX5RDy1D0flY\npxjSbTVmKWMIaqznKYfQO88Oc1kpqZB0X6p3XT3JnCkp9wXhEidc/qVY7/nUn0/9\nU1ye\n-----END CERTIFICATE-----"
    },
    "3443": {
      "key":"-----BEGIN PRIVATE KEY-----\n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\nYUQmiK6nOA0nP4T5x0A6LTbN6AOcxeg=\n-----END PRIVATE KEY-----",
      "cert":"-----BEGIN CERTIFICATE-----\nMIIFDzCCAvegAwIBAgIURfAGhgnCVBovG1GXafPioc7ln/kwDQYJKoZIhvcNAQEL\nBQAwFzEVMBMGA1UEAwwMdGVzdC5hbGIuY29tMB4XDTIxMDkyNjAzMTkxOVoXDTMx\nMDkyNDAzMTkxOVowFzEVMBMGA1UEAwwMdGVzdC5hbGIuY29tMIICIjANBgkqhkiG\n9w0BAQEFAAOCAg8AMIICCgKCAgEAr1AlmjGNbn3pVxonTlur42btuJYxkhC+P3Vt\ntZuVNKR3LxMPlMCCJ1jnJA3y9IVzh1KuJvZQExEC1DXyOhFdQHEweLLic7NSgNEa\nBFI4gEmU56W/StjR10BBiPwqTDLRR3/rVNfEkwYPc103J7QZpBWIzLpxSPdIpnPa\nfLndROjvuXZoQsZPlQwrUQg19DKWQrCsnzNBGD1LqRajOIeY0AUmied8sMXkd0rq\nIRVe1fJbIUIvYCtMHAIS4R1S69mCdy18wdBpYfrpPamOEs3L9gtZxBeEPFLiOz7+\n6jaQI0lXJ5l3+364rOykjdYa7PRpRUPP7GkP6KBBto2x4YGKPS/UuunqL50GWzjx\naM/QCs1RyekDJCR2+4BAE79Ke2HLx6d9dj6M+Kq3vCTIsfFDR6f5kkAKnXJuNTbP\nkoIx+Gg2uwbNiPSGBKdNZ0QWRYZDG3sb+cTyFL9RifawYnncWPVKsYxeCTgc+wcm\nwMh90gzcrrkGwVAIO6gvFQFiBpOFuc0BLgfhPZ0t9BWHC3epEKuKKXwbPZbQzppp\n72wkr/pdmPngBL9gSye0O+4qL3Aoy/T/NbemkwIFXToOm+hX6Nea9QafItWN5LSg\n13fGht87+mciNZ2xTXpcJvcZ6MdtqHH8/+1iaZLleO8gCaLa55NC3ieIghiSLnnz\ntKw4mDECAwEAAaNTMFEwHQYDVR0OBBYEFJWEB7GtSJ1glNCtoJLEnRtbviL2MB8G\nA1UdIwQYMBaAFJWEB7GtSJ1glNCtoJLEnRtbviL2MA8GA1UdEwEB/wQFMAMBAf8w\nDQYJKoZIhvcNAQELBQADggIBAEkeG+Kiar98U9TB4IZpiqo2dw38Zk8fcPJK6wIT\n5104F07DCErYcBp4LXWTJX9iVsdkiiVSE/FmqQjWeX5kACUuM8HkeziQNj++EcTq\nDQtDk9zEBWGDYQH4RQZvIVQlIaieZOArSbhunIrxlGr6fX//ryLO5K4QAay/oqwb\nUXFYfQ0M7VdrsTwLRImQN5KYAJbdR/Nlm5i/fTJps5iSdgpovBAommb1XpVq/aJ5\n3tAqb64vBNigZ7T8V1cCh6oDoCO/xzoucTzF9e14LkTmtzYxBjhplgwSUH6R0cgi\ndiexT2mdBtU79iJ0K5AJFVa1UCR0OE3/FmWEkb4L01XxU5sEyYL6I0JPSXaDdjtL\nv3y2GZY2Iz27qjz/JSZXoyf28rAYE0YHI3nX1wBwDTSoPKnfSc1A/IFsXFfkGB00\nuFNiI5rRff+zBt0XCAEz2Q9aULI5Ho8kjdOqHT/ty6c9RbxnJHv3mRQy0kZsb8QM\nDHTqwEvHE7mwGtd5LD5z6SRQpCfQmoSDuNqUdxMLNIYn45+BZyAlHE6Le21fH/Rb\nCjWQ5fBl7QdBHGB9dpYu8dhdrOlN0xj1QJKJGrVkqOA4nGmD6GThBX5RDy1D0flY\npxjSbTVmKWMIaqznKYfQO88Oc1kpqZB0X6p3XT3JnCkp9wXhEidc/qVY7/nUn0/9\nU1ye\n-----END CERTIFICATE-----"
    }
  },
  "http": {
    "tcp":{
        "443": [
            {
              "rule": "test-rule-1",
              "internal_dsl": [["STARTS_WITH","URL","/"]],
              "upstream": "test-upstream-2"
            }
        ],
        "3443": [
            {
              "rule": "test-rule-1",
              "internal_dsl": [["STARTS_WITH","URL","/"]],
              "upstream": "test-upstream-2"
            }
        ],
        "80": [
            {
              "rule": "test-rule-2",
              "internal_dsl": [["STARTS_WITH","URL","/"]],
              "upstream": "test-upstream-2"
            }
        ]
    }
  },
  "backend_group": [
    {
      "name": "test-upstream-1",
      "session_affinity_policy": "",
      "session_affinity_attribute": "",
      "mode": "http",
      "backends": [
        {
          "address": "127.0.0.1",
          "port":8443,
          "weight": 100
        }
      ]
    },
    {
      "name": "test-upstream-2",
      "session_affinity_policy": "",
      "session_affinity_attribute": "",
      "mode": "http",
      "backends": [
        {
          "address": "127.0.0.1",
          "port":880,
          "weight": 100
        }
      ]
    }
  ]
}
_EOC_

no_shuffle();
no_root_location();
run_tests();
__DATA__

=== TEST 1: test upgrade
--- timeout: 100
--- certificate eval: $::cert
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test_eval: require('e2e.https_upgrade.test').test()
