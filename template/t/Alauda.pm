use feature 'signatures';

package t::Alauda;
use strict;
use warnings;
use File::Basename;
use t::AlaudaLib qw(gen_main_config gen_ngx_tmpl_via_block gen_http_only gen_lua_test);
use Test::Nginx::Util;

my $ALB_BASE = $ENV{'TEST_BASE'};

log_level("info");
Test::Nginx::Util::add_block_preprocessor(sub {

    tg_log("base is $ALB_BASE");
    system("mkdir -p $ALB_BASE/logs");

    my $block = shift;
    if (defined $block->no_response_code) {
        $block->set_value("error_code",'');
    }
    if (defined $block->certificate) {
        generate_certificate($block->certificate);
    }

    if (!defined $block->timeout) {
        $block->set_value("timeout", 99999);
    }

    if (defined $block->server_port) {
        server_port_for_client($block->server_port);
    }

    if (!defined $block->request) {
        $block->set_value("request","GET /t");
    }

   	if (!defined  $block->response_body and not defined $block->response) {
		$block->set_value("response_body","ok");
	}

    # config是放在test-nginx的默认端口下
    if (defined $block->config) {
        $block->set_value("config",$block->config);
    }else {
        $block->set_value("config","");
    }
    my $default_policy = <<__END;
        {
            "certificate_map": {},
            "http": {},
            "backend_group":[]
        }
__END
    write_file($block->policy // $default_policy,"$ALB_BASE/policy.new");

    if (gen_lua_test($block) ne "") {
        tg_log("use lua test port");
        server_port_for_client(1999);
    }

    my $ngx_cfg = gen_ngx_tmpl_via_block($block);
    # write_file($ngx_cfg,"$ALB_BASE/ngx.yaml");
    my $main_config = gen_main_config($ngx_cfg);
    my $httpx_config = gen_http_only($ngx_cfg);
    # write_file($main_config,"$ALB_BASE/main.conf");
    # write_file($httpx_config,"$ALB_BASE/http.conf");
    $block->set_value("main_config",$main_config);
    $block->set_value("http_config",$httpx_config);
});

sub write_file($content,$file) {
    open(FH,'>',$file) or die $!;
    print FH $content;
    close(FH);
}

sub get_test_name($file) {
    my $dirname = dirname($file);
    my $ALB_BASEname = basename($file, qr/\.[^.]*$/);
    $ALB_BASEname=~s/.t//;
    $file = "$dirname/$ALB_BASEname";
    $file =~ m{^.*?/t/(.*)$};  # 匹配/t/后面的部分，捕获到$1中
    my $suffix = $1;  # 获取捕获的后缀部分
    $suffix =~ s{/}{.}g;  # 将后缀部分中的/替换为.
    # warn "suffix is $suffix base $ALB_BASEname \n";
    return $suffix;
}

# 我们有时mock的backend是https的需要证书，通过 --- certificate  $crt_path $key_path 就会在指定的路径下生成证书
sub generate_certificate($certificate) {
    if (defined $certificate) {
        my @certs = split /\s+/, $certificate;
        my $crt = $certs[0];
        my $key = $certs[1];
        unless (-e "$ALB_BASE/cert/tls.key") {
            my $cmd="openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes -keyout $key -out  $crt -subj \"/CN=test.com\"";
            tg_log("gen cert cmd $cmd");
            system($cmd);
        } else {
            tg_log("cert already exists");
        }
    }
}



sub tg_log(@msgs) {
   warn "[tg_log]  @msgs\n";
}

return 1;