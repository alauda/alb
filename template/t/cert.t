
use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;

# openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes   -keyout private.key -out public.cert -subj "/CN=a.com";openssl rsa -in ./private.key -out private_rsa.key

# perl -pe 's/\n/\\n/' 
# $ENV{"DEFAULT-SSL-STRATEGY"} ||= "Always";
# $ENV{"INGRESS_HTTPS_PORT"} ||= "443";

my $a_com_cert = "-----BEGIN CERTIFICATE-----\nMIIFKDCCAxCgAwIBAgIUE4OwF7BMcDRHF56HkVYPbMCCW2QwDQYJKoZIhvcNAQEL\nBQAwEDEOMAwGA1UEAwwFYS5jb20wHhcNMjIwNTE5MDcyODI3WhcNMzIwNTE2MDcy\nODI3WjAQMQ4wDAYDVQQDDAVhLmNvbTCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCC\nAgoCggIBALL3x/o0hJJas8oaywzYGIc2wdbeQ4/yuoK/dsPMkSbbZ75FmEcaqozh\nYJSb6JdE8XKNR8mJbvog8knbsoWklo/fNaQbinXEcbsGgSwjOntSB091K9Qiet4o\n08zbf9xQcv1T3qE4STism6PnzP5trDyYPJ0Jox/Gi+HMF3vXGbTqGsUOSxTxq9d1\nCLYb62kPG1ums45XgF/m08F0HDkCfh2OonV5R3jPLEleEQ8GmSIrjPfnEUkfS3sw\nGkfByuIomK635grptZqPHgtse4ZMBZmUkEwy8eKX+8mnKCdnSgkVugUISWu5NdgI\ngjpyhrZgPp5hEV6k1EzZmZxqYx0DCzy2BfmpKFe5bkKt64ADQlSqPz3OsuzKXuye\nDr3yCxy8Gcy7LEjVs0umoth+DSwAvsNeIaVxgpxUd68txF7pVNkNEOdjqc32ZYS5\njbUYa2S+irsKuJ7Y4j9CZIGLDwGzKbfkXslLL7+Z6ePa7mrIwC0fYNEzj9zibP7M\n6LCK/aLWMp2M6E0jUHBizH9gqp00lheM2GgCKdXG+sCTwecJzkEkv44V33RoUWMR\nYNwR/yM+VA91YLfXaJPk75bcysbAX41ghfIVEi4fuVRd0IaTR3FC4XCvUcTayL5m\nmgFzX7QtSQ0EIrrZRSI6zjCkiRKpMBx0BJBowvdtcp9Y13cQxFZBAgMBAAGjejB4\nMB0GA1UdDgQWBBQl9RFLX4soAfwgvi0v3Rd3XAoc4TAfBgNVHSMEGDAWgBQl9RFL\nX4soAfwgvi0v3Rd3XAoc4TAPBgNVHRMBAf8EBTADAQH/MCUGA1UdEQQeMByCFGZh\na2UuMTAuMTk5LjAuMTcuY29thwQKxwARMA0GCSqGSIb3DQEBCwUAA4ICAQBIDr2R\ngxW5JbqyOaw7U1YJ8omImHhyDExFubNf3b93/oKVpbebMl4dJD82I6lnFUPcTC7P\novOXGF/zvo613uwIv7p8YxMgG6XwhdYkwsWFUUZttyHUZU7Bvyx46Q4wFv/V4Egp\nsxYuS4wrMg9peb7NuCJb6fLfQXUJdh+7MDbjV2x3dgELlscSHKJYiDXNauXIv/Kx\nI7rlFRfyJ14u7SP+BhexpuxSaVki3PXD6549bqtQUPsj+kuRNAN31Ymv81tPDa42\n7aPOrQqs7xt9KhBjhgOQsZg4jJvfWCgfQ+Ym037jylIqXuxrAE4w8hCt2o2wz0ux\nY1bhXkq9DBeeK1jYdvGQx8tuyT8QP4wD1rcj4yHEvK2kRU+P7FLQ/EastoR1nm+i\nn4mNaJCpLfeuTEGTWLTO/DPFtXgkmv4emlj9RNlHqrd4BTbLqNef/vxhVohdZ0b/\nJGrYZAWsg81olPkt0hHYnjHcGbhbCBuZeeh0EwABuFBbv9F8TidE3IZ2Vy6w8hmt\nA6eKrroiANcUisH9nhXK42k5+veaaZIeWzLoTpy2DFXV0hq+SjYGNDOLyVT5Hx3J\njblhqg8IqW4oZuGW7Z9KE+9sAzZhfolJ7NiN4yy95qu3NUaFRIMu2WphZXAmO2Td\nluJ6ImHGDB0dohMMJslXEMUusZ/ghc/VeZffMA==\n-----END CERTIFICATE-----";
my $a_com_key = "-----BEGIN RSA PRIVATE KEY-----\nMIIJKAIBAAKCAgEAsvfH+jSEklqzyhrLDNgYhzbB1t5Dj/K6gr92w8yRJttnvkWY\nRxqqjOFglJvol0Txco1HyYlu+iDySduyhaSWj981pBuKdcRxuwaBLCM6e1IHT3Ur\n1CJ63ijTzNt/3FBy/VPeoThJOKybo+fM/m2sPJg8nQmjH8aL4cwXe9cZtOoaxQ5L\nFPGr13UIthvraQ8bW6azjleAX+bTwXQcOQJ+HY6idXlHeM8sSV4RDwaZIiuM9+cR\nSR9LezAaR8HK4iiYrrfmCum1mo8eC2x7hkwFmZSQTDLx4pf7yacoJ2dKCRW6BQhJ\na7k12AiCOnKGtmA+nmERXqTUTNmZnGpjHQMLPLYF+akoV7luQq3rgANCVKo/Pc6y\n7Mpe7J4OvfILHLwZzLssSNWzS6ai2H4NLAC+w14hpXGCnFR3ry3EXulU2Q0Q52Op\nzfZlhLmNtRhrZL6Kuwq4ntjiP0JkgYsPAbMpt+ReyUsvv5np49ruasjALR9g0TOP\n3OJs/szosIr9otYynYzoTSNQcGLMf2CqnTSWF4zYaAIp1cb6wJPB5wnOQSS/jhXf\ndGhRYxFg3BH/Iz5UD3Vgt9dok+TvltzKxsBfjWCF8hUSLh+5VF3QhpNHcULhcK9R\nxNrIvmaaAXNftC1JDQQiutlFIjrOMKSJEqkwHHQEkGjC921yn1jXdxDEVkECAwEA\nAQKCAgAYo9KtmRNzjvdX6Q5xq0LdQuW3LozAwdt56uBwHrcRUX3cDXrkt0Ap+1Gv\nxDNmuEBB1D/A+KIF4Albr9rJWZq9Hi8ldAFBK5W4+TFJoWQI3IdTIj+xijm+YoKe\nns3gyFa8mBJ7weMa4XDgRSbNFM503UTjHhOOaWiS4uWM0FWiueSLouclcAyHsn5L\njFaB9Wl/2di4zUVIbuBSryi/lJ9GdH/biqITePqQ81mH5xGoSbSz4OVZWuyqfjnw\nDTdgodQ7oegTMpAlQnURf5MWL1tKBNFFHHJ/DwvEfLYjjq37yDj/Pl/Va/+Eyc8c\nOu5fJ6sXZSfeDvWHyyHCDketE+E09KbxGzuk556zg+04zB5Ij08X9M6jwQV4rWgO\nGgxvHmTVreNyiEJP8cCpZmYmfHlgnacEGskIjSgqj7eVXdgyKZPefKxUjIoUhlzJ\nEP0gx02kqRRfMxzKTVxmnZrhnGZrcIrbIlduz3plf6xKA2nAipD1GAgxV5ACMvBE\n4ISm17mJ7PDGgPYzD4ST8Itkq8MMkC/1E8JVAEFYc9zArcDvMlRr7uO0Tua4uc0v\nBZICFl7m7B2WiLf7/p8SMDr45v9Mvtf55duQx5bripwoBX7N+DvzLrpwKbGpulTm\nqkSrM0Ts+AslbutKyRvC1/f6M9kMVyAQllRUhtZM5C5csr6AAQKCAQEA3LpIzI6g\npmEcpIvzPM3pA3jU8wlkCIKK4kcEOHpMgq/rbLOZOYGZikEqmAYDoKuM40tgmXS3\nFMktlemU//Rsj07hZDyXNmvBrqO4tf4I7r/WXcKxOHENNt/Sw1vRGwmHgLmO2c56\n9rjNZDOJBrMFPJDbgf/0XBP9HRfefTGYN5uwIshuLStKb+epas6dDzc6LXVmmG+Q\n1meBs27WVf8nSGIwPqLiEawepRrvF6kB/9V37ZYcJplk3zPgaCvfhk4DLdV42A0x\nH42Bw1jc6Q7MUayFakF/2YixI+n+dwb0YPDVoymk6go4CgGO+AlkHZkfLTR29yuT\neyjFPHqmq930QQKCAQEAz5Em+BmVY7ISIUxZlSRdMX7KkuZRwPJSY0RvIdE9xfDt\nrz6YwCyVQTgx99u5OYV+D7cJzr6O2c5Y98arrvFgNm4Rq+7lTDqIf68hXAm5Fh1X\n2F5qBiCiwy2jZxuXqVi9JLyH5IaLZky5ATDrBAXWTsDgIZhEdPIbu9UGWXMy3lvd\n8DC9WSSqT9nB+6qKy5TRpfYbRJO/PAjiSvC9unzEXmAyQK1Q7IifSI4xrF6YKyUn\nvpdvZLN48UZ09u2wNuna07U/VgCEC/Q9AvEFHhjiBqE7M+GEA+TLsW8KPoAlAK/v\ng3hR/vv+hZ2tZit4E3v5U/dKCJ4XHEirBzCdLgXiAQKCAQB+92Dc2cYrLn1NYXtf\nJIq+hojn7CTwiDbfhj41RpQwMIVZl82xuIzbbDTWEc+QYl2+eSNt4idV+4sPSrd8\nq9qubI9WG0xX75APpvmfJit5OjxS3qUWdGFHiWQxH+WeidK6BwLW4uD0fsUWuFY/\n1kZS2niJxPOI666TR6Ghnh+TDSk6ONS3gslkqXtYhtTtZbU/ZOLJGJPV4OBImJ8O\nBKFSD7j0rrkftURDcMTLdVpDEUXVEp3Kzj2p7qtNAL+o/8LwYHUMwjnZjopwFfOs\n0+hPqs9rmZWzSd+ravQG/6cfBCm/mzrTrWEi0Fau8qf2Jpg6Zo1wDE7fb0pVSbAJ\n+LiBAoIBAFQVBKg0FOQR2m5Ks29LD8VhC0Z+rldu0hkMO8iDLnbkpiP7Q311kfCd\nhwBUra+zd+F90CdD4jIw+LFGdX2kocjqxZXUbGZ4v5qZovXZqnRe5prrhB9/UO+n\nqS23a7RaEiSzioj0R7vlEHx/CHTUuH+meiShvflxqfJo1O2fUNfqdvk5hTp7M9Ks\n73u3Fgpp+pM0Is+g2jLDloetBe5pZFKmvTSeAM4QehW2JEEjAJlZr8PxLFqqqS9z\nzyXIGz3jdZWVMlbwVo1RHvX2FJCgm877uTPHAudg43K4/HldB6BDpM6pCu4zvmL6\nAKgGq9mYuuNcpUzgXZRDi6SZ+NIP6AECggEBANPDmzRJa1LnyHt8sXsMoMGMC5bi\ndY1bXogWUzm9yhQOC5uqM0E5+TGltU5f7DxArEKLoCURIkkiR0STcHJOPckZO3mu\np23d60gq0s8V7r5TckLCBtl98Cc8asz/cvNqljBwD7QHlIHHZXyna7OE406qq/mg\nfjVCN6YvjMlc+/CYXp9EPFihRX0IhIsnVafBHDLA7oZOUbzJqsEGnLqqX4gQOUIY\nV7uduGYaXd6NLrKKhLnZ6emsE00HrmQWbpD+NqLnjqgqcNyvONNJ3DdLmiMTV+hK\n1S0mxq8RqdZWah3+zus9DshNcLkZszYz5aU5CPjM9hLsev+3nzDhSD/LND0=\n-----END RSA PRIVATE KEY-----";

my $default_443_cert = "-----BEGIN CERTIFICATE-----\nMIIFFTCCAv2gAwIBAgIUNcaMWCswms56XCvj8nxC/5AKxtUwDQYJKoZIhvcNAQEL\nBQAwGjEYMBYGA1UEAwwPNDQzLmRlZmF1bHQuY29tMB4XDTIyMDUxOTA5MjEzMVoX\nDTMyMDUxNjA5MjEzMVowGjEYMBYGA1UEAwwPNDQzLmRlZmF1bHQuY29tMIICIjAN\nBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAvMqeEYs9K/DrbZ3FXj7yZaVRexub\na4OF0S/jg2qXrTK8FwPQ1MJiVxPL2jNeE7PT1fCNujb3+fZ/way99FJ0KpmbqEeP\nGt8490oqHZl7LuiEklrSiNp5qOJERsxgNrtq5RILIC1wH9eu0dNilwnCldzEXqFJ\n4+vZqPQfNxk//0vSOxsapl/nEPze6aMy+sUnyFJoq3ti/O02sV/p5sOQX3NcPoXU\n23PTr1xMDVQ7IpuR4GkxbmIVdAMuGWA2udYN0H3ou1VVy+je3RVF7xD2V/lMI3RL\nzLinfWxyNBUOoswylWRjdgwfrz5EkGuN58uT+o28Lx0APw06gwZ0eXb0cKdaYM0X\n04p4d3r//KLgm5WZpvDrjCC3aP02Yk1rITAu9owx+fNjIuEuPJtfin6r9Cjed7xL\n9CgdlFONDkNPxMz52qnf9Jbuf4HPTa/jDw7ICG8FAR8RwljJ7ohFCmullfXtumpX\nbT8+4DK1+H1fqkLV4lCWtQwn8ULqqCDQJZszco5KcnNnenqKgNPLVe7t6/ZxDZ8j\nMYAyGIR+DMDp0tLjfHD26IjEzF/n3E0pZiTXFaRirjKcFd523qEWvZeZc0nSykhH\nBUYXgxh2Nqi3Cv9VxA6sHVto5GvQBWq0kl6Qo9IGof51+4HCm++8/bCpf2Gcv/kI\ny39JIHMSGCa4ztcCAwEAAaNTMFEwHQYDVR0OBBYEFDawzxBvztJOekhp/DU9GKo+\nsnc9MB8GA1UdIwQYMBaAFDawzxBvztJOekhp/DU9GKo+snc9MA8GA1UdEwEB/wQF\nMAMBAf8wDQYJKoZIhvcNAQELBQADggIBACd2Z9XyESvQ4MfYMUID2DCmuVGBDhyo\n8cN88nuy+plrcYSpsthp55C+dhhfJRocES0NkpIVojpiQiPQdAyLFKb1M1Mcd9bg\n+qtYrOH2lS0Uem2s366D8LLJSOzWv/f75wUHe3eyivzW73zcM3znr5TrAFrCkUBF\npkK90G1VEznpD+VDvXYfcXklTZ7lMVZJ1ck2MDYPkh3nGtCyY6z+r41vJo/OcW8A\ncxicgsKXjEiXOH42B8ugad5gK27gA/FKwtTNPPU4K0UeDCAJaY+L7USjbrUgeQ17\nmjCOrY53OjyjjD4YjsE9EqsU/Hc9lqIUdCktZEDrLKfjGT1raaqDlSzEYYcs/oai\n0Ka3MXao2czYEJz6YZIOtp7FatRUBajCZ3NJeTgPFMZn10g7CktJR5QJDvvqbUBs\nHCddmahNPdgQwjxGVfoAI5SDH2QnIlj3bLivU+4oqR7hO7Nmhx9BtNRdHhM+M+wp\nsLvVETvtZdHC3RX4rX4pAl/r7pjhC7n0tbn3XyK96yZ4Yu/E+d/Cqhs0+rssqLzH\nDtMZCMOsaZi1AUEtc2cmZweOXEHeEoyPn3nJeVLfW2+dThlK/i9RaZbPThTS/GdK\nCU530BEDG+y/I5p6dndYySm2+LJiC0Xso1S1gLa7NccV8Y1E9Y8026J3lpvMilhP\nBwA4jE77yBPI\n-----END CERTIFICATE-----";
my $default_443_key = "-----BEGIN RSA PRIVATE KEY-----\n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\n-----END RSA PRIVATE KEY-----";

# wildcard cert *.alauda.cn copy from dex.tls, expire in 2023-05-10.
my $alauda_cert = "-----BEGIN CERTIFICATE-----\nMIIGhjCCBW6gAwIBAgIQCZhrm1vPSv7Cvd4oftN4zDANBgkqhkiG9w0BAQsFADBZ\nMQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMTMwMQYDVQQDEypS\nYXBpZFNTTCBUTFMgRFYgUlNBIE1peGVkIFNIQTI1NiAyMDIwIENBLTEwHhcNMjEx\nMjE1MDAwMDAwWhcNMjIxMjE1MjM1OTU5WjAWMRQwEgYDVQQDDAsqLmFsYXVkYS5j\nbjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALtXHkgAOGypTXFZ9ZDH\nurxo/yGIjYxO03948BTo96UfFEYKAPUU93oZ2fdnoWHplkKM9QiXlBjjwSng4KFb\nFJ43ZXdDg3IR/hk6SfoqDlmbMclf5LaV0SCozLIeCH5BHlhQbzhwHk6XqAEjoTLv\n2MHCNH4X/TOjy1Ga7RKrEtfsZRDgSpo/vxZbD99a7/B7ccPIWvCKDP1yIlduwgq+\nuS5hfLduv2PyL29zi+ihuK3TMBi9SmApbaEN4XzBD8RsPUaL5BISrOuqunArCYDH\nb0rrLP7ylKpq08nTTp2IDORspkurv48cnhrc3gZwAHEPh9EEwNRcFELCp+5awvmZ\nE+0CAwEAAaOCA4swggOHMB8GA1UdIwQYMBaAFKSN5b58eeRwI20uKTStI1jc9TF/\nMB0GA1UdDgQWBBQc9AQopIaUdcVloegrCtGgJfLabjAhBgNVHREEGjAYggsqLmFs\nYXVkYS5jboIJYWxhdWRhLmNuMA4GA1UdDwEB/wQEAwIFoDAdBgNVHSUEFjAUBggr\nBgEFBQcDAQYIKwYBBQUHAwIwgZsGA1UdHwSBkzCBkDBGoESgQoZAaHR0cDovL2Ny\nbDMuZGlnaWNlcnQuY29tL1JhcGlkU1NMVExTRFZSU0FNaXhlZFNIQTI1NjIwMjBD\nQS0xLmNybDBGoESgQoZAaHR0cDovL2NybDQuZGlnaWNlcnQuY29tL1JhcGlkU1NM\nVExTRFZSU0FNaXhlZFNIQTI1NjIwMjBDQS0xLmNybDA+BgNVHSAENzA1MDMGBmeB\nDAECATApMCcGCCsGAQUFBwIBFhtodHRwOi8vd3d3LmRpZ2ljZXJ0LmNvbS9DUFMw\ngYUGCCsGAQUFBwEBBHkwdzAkBggrBgEFBQcwAYYYaHR0cDovL29jc3AuZGlnaWNl\ncnQuY29tME8GCCsGAQUFBzAChkNodHRwOi8vY2FjZXJ0cy5kaWdpY2VydC5jb20v\nUmFwaWRTU0xUTFNEVlJTQU1peGVkU0hBMjU2MjAyMENBLTEuY3J0MAkGA1UdEwQC\nMAAwggGABgorBgEEAdZ5AgQCBIIBcASCAWwBagB3ACl5vvCeOTkh8FZzn2Old+W+\nV32cYAr4+U1dJlwlXceEAAABfb5NwVgAAAQDAEgwRgIhANFWCollyinyptcck56g\nvBe2sWcylnpH4Ox8njakGgAFAiEAuqPM1+qdjBGdoqeLIz2Z+XGQQ2ExNSLhzSa7\nc9VgTcUAdwBRo7D1/QF5nFZtuDd4jwykeswbJ8v3nohCmg3+1IsF5QAAAX2+TcGC\nAAAEAwBIMEYCIQCksk3gsDrrb+X0v5KxHFzSmS1KT2/RR6lySF8bG3GLLQIhALye\ndJhLT75nvXX0xAcVWUzkgheT6uyOVJHhOOl7NlqiAHYAQcjKsd8iRkoQxqE6CUKH\nXk4xixsD6+tLx2jwkGKWBvYAAAF9vk3BNgAABAMARzBFAiEAqnQvEzk5SgCXWSU9\nx3EM+UjygI4WUsOF5w4svk2t03ECIB+c6XhSZQbMVbACeM9Wy8UZNAGWmtLAM6yn\n63Kj7xrcMA0GCSqGSIb3DQEBCwUAA4IBAQADxHLHUJ/VwrIbE6cVsswXlm+AoHu0\n2YMqgaPIJW/YUzz6S7FXPh7py9h4QF5m03taVZBtxMGD7EOARWJ4NlSPl7FXH0TA\nPG5HMIu/9ymQKjK2vPXApnkLKtIoHxO3feemVkyLZY2EZJZ3KuRHQlZ5A/SnPtn+\ny3eXtGdmsjEw3ymlltzEJUB8U3BDNBa9Fh0iP/Op4QUPWIoxO4mPMzjcVvAoIdie\nTiVWmsoFnd2u6E34KBdqcWE6SgD8lMeSGKyuGLUej0Qk+X3XaXFBiZHPEsoowl/u\nBMUdLi8nmuBt5lRd58qSKC3AL1ViT1cMRXh/tvp+QgWKp2ntQogXmRsU\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIFUTCCBDmgAwIBAgIQB5g2A63jmQghnKAMJ7yKbDANBgkqhkiG9w0BAQsFADBh\nMQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3\nd3cuZGlnaWNlcnQuY29tMSAwHgYDVQQDExdEaWdpQ2VydCBHbG9iYWwgUm9vdCBD\nQTAeFw0yMDA3MTYxMjI1MjdaFw0yMzA1MzEyMzU5NTlaMFkxCzAJBgNVBAYTAlVT\nMRUwEwYDVQQKEwxEaWdpQ2VydCBJbmMxMzAxBgNVBAMTKlJhcGlkU1NMIFRMUyBE\nViBSU0EgTWl4ZWQgU0hBMjU2IDIwMjAgQ0EtMTCCASIwDQYJKoZIhvcNAQEBBQAD\nggEPADCCAQoCggEBANpuQ1VVmXvZlaJmxGVYotAMFzoApohbJAeNpzN+49LbgkrM\nLv2tblII8H43vN7UFumxV7lJdPwLP22qa0sV9cwCr6QZoGEobda+4pufG0aSfHQC\nQhulaqKpPcYYOPjTwgqJA84AFYj8l/IeQ8n01VyCurMIHA478ts2G6GGtEx0ucnE\nfV2QHUL64EC2yh7ybboo5v8nFWV4lx/xcfxoxkFTVnAIRgHrH2vUdOiV9slOix3z\n5KPs2rK2bbach8Sh5GSkgp2HRoS/my0tCq1vjyLJeP0aNwPd3rk5O8LiffLev9j+\nUKZo0tt0VvTLkdGmSN4h1mVY6DnGfOwp1C5SK0MCAwEAAaOCAgswggIHMB0GA1Ud\nDgQWBBSkjeW+fHnkcCNtLik0rSNY3PUxfzAfBgNVHSMEGDAWgBQD3lA1VtFMu2bw\no+IbG8OXsj3RVTAOBgNVHQ8BAf8EBAMCAYYwHQYDVR0lBBYwFAYIKwYBBQUHAwEG\nCCsGAQUFBwMCMBIGA1UdEwEB/wQIMAYBAf8CAQAwNAYIKwYBBQUHAQEEKDAmMCQG\nCCsGAQUFBzABhhhodHRwOi8vb2NzcC5kaWdpY2VydC5jb20wewYDVR0fBHQwcjA3\noDWgM4YxaHR0cDovL2NybDMuZGlnaWNlcnQuY29tL0RpZ2lDZXJ0R2xvYmFsUm9v\ndENBLmNybDA3oDWgM4YxaHR0cDovL2NybDQuZGlnaWNlcnQuY29tL0RpZ2lDZXJ0\nR2xvYmFsUm9vdENBLmNybDCBzgYDVR0gBIHGMIHDMIHABgRVHSAAMIG3MCgGCCsG\nAQUFBwIBFhxodHRwczovL3d3dy5kaWdpY2VydC5jb20vQ1BTMIGKBggrBgEFBQcC\nAjB+DHxBbnkgdXNlIG9mIHRoaXMgQ2VydGlmaWNhdGUgY29uc3RpdHV0ZXMgYWNj\nZXB0YW5jZSBvZiB0aGUgUmVseWluZyBQYXJ0eSBBZ3JlZW1lbnQgbG9jYXRlZCBh\ndCBodHRwczovL3d3dy5kaWdpY2VydC5jb20vcnBhLXVhMA0GCSqGSIb3DQEBCwUA\nA4IBAQAi49xtSOuOygBycy50quCThG45xIdUAsQCaXFVRa9asPaB/jLINXJL3qV9\nJ0Gh2bZM0k4yOMeAMZ57smP6JkcJihhOFlfQa18aljd+xNc6b+GX6oFcCHGr+gsE\nyPM8qvlKGxc5T5eHVzV6jpjpyzl6VEKpaxH6gdGVpQVgjkOR9yY9XAUlFnzlOCpq\nsm7r2ZUKpDfrhUnVzX2nSM15XSj48rVBBAnGJWkLPijlACd3sWFMVUiKRz1C5PZy\nel2l7J/W4d99KFLSYgoy5GDmARpwLc//fXfkr40nMY8ibCmxCsjXQTe0fJbtrrLL\nyWQlk9VDV296EI/kQOJNLVEkJ54P\n-----END CERTIFICATE-----";
my $alauda_key = "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAu1ceSAA4bKlNcVn1kMe6vGj/IYiNjE7Tf3jwFOj3pR8URgoA\n9RT3ehnZ92ehYemWQoz1CJeUGOPBKeDgoVsUnjdld0ODchH+GTpJ+ioOWZsxyV/k\ntpXRIKjMsh4IfkEeWFBvOHAeTpeoASOhMu/YwcI0fhf9M6PLUZrtEqsS1+xlEOBK\nmj+/FlsP31rv8Htxw8ha8IoM/XIiV27CCr65LmF8t26/Y/Ivb3OL6KG4rdMwGL1K\nYCltoQ3hfMEPxGw9RovkEhKs66q6cCsJgMdvSuss/vKUqmrTydNOnYgM5GymS6u/\njxyeGtzeBnAAcQ+H0QTA1FwUQsKn7lrC+ZkT7QIDAQABAoIBAB2aBO9klYXZ9KIy\nEELZxHBr+NBgJtmiRPoR7oGnVCYztHzirMcNEpOpDQ9yQQZbJgKLClbauKx8JHQN\nFAF7BlV/tFk1gkoefLOYycKtLYpMIwBKVjXhk2NhOML2SupEONrEjuZwlOFfRk0z\nx49oZawsFyZLfRdRTNmurMIz5OzYKBI20fq2zrcduYCuFvOocZbwMU3LVz4ZSFLR\nl3/4/xdtgsfVS0ENoYOp+EMZbTVc8dzNTDQg6a/r/O0hYgwtqD31KfgrsEdjzYMs\noQLyNe8Fr5yKN2VusjLVwlBY93WCLR82ReXoivO2xx92Y0SxLLTpF0l6+1WTD0i+\nQFzCGqMCgYEA/3cNXtOxjnX3DJToCkrMJkMZo3q6AFXbq/m0q9XesGcyzFXu4TwW\nGTxrVNEFjnZHBWxeieXKOk8fgQmUjlsmcx+I4glvGrgVFF6UZE/CmrTq2LKfY9WY\n3MKGMMMaWaGtVum0BYeUFVFUW1s6cMxCHStjTz6W2oVcUtOdXM12PWMCgYEAu7uL\n18Z8jZrOICFiepxeEUM00q20I2l2fMqG3nVvPg3VOvWNfKvsSi4/unicd2B+jVd8\nbfK/qsCHgN5oF2rvrOqxxqhYmvzpZOErJy+UXUzygcE5Jglv1+qKCcoUtXYm0Hq3\nVnK2oWDiBHYYdWZ4Kwy5D1Q6W+cxiBJuwiwGkm8CgYEAsE/LQ4oZPjg+REm1B/1t\nfm7LECAQpVCcZsnVHs9hfSAMWChq0Lp2if5AGW6VRihthdmwOb4FX07icF1bURCp\nrcSy5UYbjzZDHibUhZLivYFloB9PkEiH0rzSfm75DalfB+ANpc9XrYrPDKoe4GCo\ntJcQWE3bMX/fIy73qWgIVf8CgYAtZmCeUREECcD5gjlXn4McN52JqZpbygBuk2fk\nWpAJeLztYj7SPJ2LHv4ocUydjgds1RBxYng5qg/a+W5A44qMzcEqYsHy0WD8FXwj\nIN2HZrlq6biRW0zh8YVqcqVpcOZYGqVF0b4a7twZ6hlmIt7CwnPqohru6M1Qs+x3\nJsB8HwKBgH4LU7OdCIH822kaQ0RjKBLGHJO65ee8xROOpq7M1h/M4mU3sTT4XFJy\nBjFMvMqiQtKxO4XmtOxUa0VHI3YF/e0xNXWu9MoMuGVFvkzheLc5IiJfnTD6Lz/f\nKuGmnsnBU3Cg1/opdjrJZMzLE4M5YDAW5wB2nChFDBnCkZdmpPII\n-----END RSA PRIVATE KEY-----";

our $policy = <<"_EOC_";
{
  "certificate_map": {
    "*.alauda.cn": {
      "key": "$alauda_key",
      "cert": "$alauda_cert"
    },
    "a.com": {
      "key": "$a_com_key",
      "cert": "$a_com_cert"
    },
    "2443": {
      "key": "$a_com_key",
      "cert": "$a_com_cert"
    },
    "443": {
      "key": "$default_443_key",
      "cert": "$default_443_cert"
    }
  },
  "http": {"tcp": {
      "443": [{
          "rule": "",
          "internal_dsl": [["STARTS_WITH","URL","/" ]],
          "upstream": "test-upstream-1"
        }] ,
      "2443": [{
          "rule": "",
          "internal_dsl": [["STARTS_WITH","URL","/" ]],
          "upstream": "test-upstream-1"
        }] }
  },
  "backend_group": [
    {
      "name": "test-upstream-1",
      "mode": "http",
      "backends": [
        {
          "address": "127.0.0.1",
          "port": 9999,
          "weight": 100
        }
      ]
    }
  ]
}
_EOC_

our $http_config = <<'_EOC_';
server {
    listen 9999;
    location / {
       content_by_lua_block {
           ngx.print("ok");
      }
    }
}
_EOC_

log_level("info");
no_shuffle();
no_root_location();
run_tests(); 

__DATA__

=== TEST 1:  cert should ok
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
    local F = require("F");local u = require("util");local h = require("test-helper");
    local shell = require "resty.shell"

    --do 
        --local ok, stdout, stderr = shell.run([[ openssl s_client -connect 127.0.0.1:443 -servername a.com |grep a.com ]])
        --h.assert_contains(stdout,"a.com")
    --end


    do
        -- wildcard hostname
        local ok, stdout, stderr = shell.run([[ openssl s_client -connect 127.0.0.1:443 -servername a.alauda.cn  ]])
        h.assert_contains(stdout,"*.alauda.cn")

        local ok, stdout, stderr = shell.run([[ openssl s_client -connect 127.0.0.1:443 -servername b.alauda.cn  ]])
        h.assert_contains(stdout,"*.alauda.cn")

        local ok, stdout, stderr = shell.run([[ openssl s_client -connect 127.0.0.1:443 -servername abc.alauda.cn  ]])
        h.assert_contains(stdout,"*.alauda.cn")
    end

    do
        -- use port cert 
        local ok, stdout, stderr = shell.run([[ openssl s_client -connect 127.0.0.1:2443   ]])
        h.assert_contains(stdout,"a.com")
    end

    do
        -- should use default 443 cert
        local ok, stdout, stderr = shell.run([[ openssl s_client -connect 127.0.0.1:443 -servername b.com  ]])
        h.assert_contains(stdout,"443.default.com")
    end
--- response_body: ok