use feature 'signatures';
package t::AlaudaLib;
use strict;
use warnings;
use File::Basename;
use YAML::PP;
use File::Temp;

use Exporter qw(import);
 
our @EXPORT_OK = qw( gen_main_config gen_ngx_tmpl_via_block gen_http_only gen_lua_test);


my $ALB_BASE = $ENV{'TEST_BASE'};

my $LUACOV = $ENV{'LUACOV'};


sub tgl_log(@msgs) {
   warn "[tgl_log]  @msgs\n";
}

tgl_log("lua cov $LUACOV");

sub gen_lua_test($block) {
    my $lua_test_mode = "false";
    my $lua_test_full = '';
    my $server_port = 1999;
    if (defined  $block->lua_test_file) {
        $server_port = 1999;
        $lua_test_mode = "true";
		my $lua_test_file=$block->lua_test_file;
        $lua_test_full = <<__END;
server {
    listen 1999;
    location /t {
        content_by_lua_block {
			local test=function()
				require("$lua_test_file").test()
			end
			test()
			ngx.print("ok")
		}
	}
}
__END
    }

    if (defined  $block->lua_test) {
		my $lua_test=$block->lua_test;
        $server_port = 1999;
        $lua_test_mode = "true";
        $lua_test_full = <<__END;
server {
    listen 1999;
    location /t {
        content_by_lua_block {
			local test=function()
				$lua_test
			end
			test()
			ngx.print("ok")
		}
	}
}
__END
	}
    if (defined  $block->lua_test_eval) {
        $server_port = 1999;
        $lua_test_mode = "true";
		my $lua_test_eval=$block->lua_test_eval;
        $lua_test_full = <<__END;
server {
    listen 1999;
    location /t {
        content_by_lua_block {
			local test = function()
				$lua_test_eval
			end
			local ok,ret = pcall(test)
            if not ok then
                ngx.log(ngx.ERR," sth wrong "..tostring(ret).."  "..tostring(ok))
			    ngx.print("fail")
                ngx.exit(ngx.ERROR)
            end
			ngx.print("ok")
		}
	}
}
__END
	} 
    return $lua_test_full;
}

sub write_file($policy,$p) {
    open(FH,'>',$p) or die $!;
    print FH $policy;
    close(FH);
}

sub gen_custom_init($block) {
	my $init_worker  = <<__END;
init_worker_by_lua_block {
    require("mock_worker_init").init_worker()
}
__END

    if (defined $block->disable_init_worker) {
        $init_worker = "";
    }
    my $init = "";
    if ($LUACOV eq "true") {
        $init = <<__END;
init_by_lua_block {
   if ngx.config.subsystem == "http" then
       require 'luacov.tick'
       jit.off()
    end
}
__END
    }

    if (defined $block->enable_nyi) {
        my $name = $block->enable_nyi;
        if ($name eq "") {
            $name = "nyi";
        }
        $init = <<__END;
init_by_lua_block {
    local detail=false
    if detail then
        local dump = require "jit.dump"
        dump.on(nil, "$ALB_BASE/.$name.nyi")
    else
        local v = require "jit.v"
        v.on("$ALB_BASE/.$name.nyi")
    end
}
__END
    }

    if (defined $block->init_worker_eval) {
        my $init_worker_eval=$block->init_worker_eval;
        my $init_worker_lua = <<__END;
init_worker_by_lua_block {
    $init_worker_eval
}
__END
        $init_worker = $init_worker_lua;
    }
    my $init_full=<<__END;
$init
$init_worker
__END
    return $init_full;
}

sub gen_mock_backend($mock_backend) {
    if (!defined $mock_backend) {
        return "";
    }
    if ($mock_backend eq "") {
        return "";
    }

    my $cfg = "";
    my @array = split ' ', $mock_backend;
    my $port = $array[0];
    my $module = $array[1];
        # tgl_log("get mock backend $port | $module");
        $cfg= <<__END;
server {
    listen $port;
    location / {
       content_by_lua_block {
            require("$module").as_backend($port)
      }
    }
}
__END
    return $cfg;
}

sub gen_env_only($ngx) {
    my $ypp = YAML::PP->new;   
    my $yaml = $ypp->load_string($ngx);
    $yaml->{flags}{showEnv} =  builtin::true;
    my $ngx_new = $ypp->dump_string($yaml);

    my $temp_file = File::Temp->new("ngx_env_only_XXXXX", SUFFIX => '.yaml');
    $temp_file->flush();
    my $temp_filepath = $temp_file->filename;
    print $temp_file $ngx_new;
    $temp_file->close();
    my $out = `ngx_gen < $temp_filepath`;
    $temp_file->close();
    return $out;
}

sub gen_root_only($ngx) {
    my $ypp = YAML::PP->new;   
    my $yaml = $ypp->load_string($ngx);
    $yaml->{flags}{showRootExtra} =  builtin::true;
    my $ngx_new = $ypp->dump_string($yaml);

    my $temp_file = File::Temp->new("ngx_root_only_XXXXX", SUFFIX => '.yaml');
    $temp_file->flush();
    my $temp_filepath = $temp_file->filename;
    print $temp_file $ngx_new;
    $temp_file->close();
    my $out = `ngx_gen < $temp_filepath`;
    $temp_file->close();
    return $out;
}

sub gen_http_only($ngx) {
    my $ypp = YAML::PP->new;   
    my $yaml = $ypp->load_string($ngx);
    $yaml->{flags}{showHttp} =  builtin::true;
    my $ngx_new = $ypp->dump_string($yaml);
    my $temp_file = File::Temp->new("ngx_http_only_XXXXX", SUFFIX => '.yaml');
    $temp_file->flush();
    my $temp_filepath = $temp_file->filename;
    print $temp_file $ngx_new;
    $temp_file->close();
    my $out = `ngx_gen < $temp_filepath`;
    $temp_file->close();
    # tgl_log("http_only $ngx_new");
    # tgl_log("http_only out $out");
    return $out;
}

sub gen_stream_only($ngx) {
    my $ypp = YAML::PP->new;   
    my $yaml = $ypp->load_string($ngx);
    $yaml->{flags}{showStream} =  builtin::true;
    my $ngx_new = $ypp->dump_string($yaml);
    my $temp_file = File::Temp->new("ngx_stream_only_XXXXX", SUFFIX => '.yaml');
    $temp_file->flush();
    my $temp_filepath = $temp_file->filename;
    print $temp_file $ngx_new;
    $temp_file->close();
    my $out = `ngx_gen < $temp_filepath`;
    $temp_file->close();
    return $out;
}

sub gen_main_config($ngx) {
    my $env_only = gen_env_only($ngx);
    my $root_only = gen_root_only($ngx);
    my $stream_only = gen_stream_only($ngx);
    my $main_config = <<__END;
$env_only
$root_only
$stream_only
__END
    # tgl_log("main_config $main_config");
    return $main_config;
}

sub gen_ngx_tmpl_via_block($block) {
    my $stream_config = $block->alb_stream_server_config // "";
    my $init_full = gen_custom_init($block);
    my $mock_backend = gen_mock_backend($block->mock_backend // "");
    my $http_config = $block->http_config // "";
    my $lua_test_full = gen_lua_test($block);
    my $default_lua_path = "/usr/local/lib/lua/?.lua;$ALB_BASE/nginx/lua/?.lua;$ALB_BASE/nginx/lua/vendor/?.lua;";
    my $lua_path= "$default_lua_path;$ALB_BASE/t/?.lua;$ALB_BASE/?.lua;$ALB_BASE/t/lib/?.lua;;";

    my $default_ngx_cfg = gen_ngx_tmpl_conf($init_full,$stream_config,$lua_path,$mock_backend,$http_config,$lua_test_full);
    $default_ngx_cfg = gen_https_port_config($block->alb_https_port // "",$default_ngx_cfg);
    $default_ngx_cfg = patch_custom_location($block,$default_ngx_cfg);
    return $default_ngx_cfg;
}

sub gen_ngx_tmpl_conf($init_full,$stream_config,$lua_path,$mock_backend,$http_config,$lua_test_full) {
    # Add 4 spaces prefix to mock_backend
    $mock_backend =~ s/^/    /gm;
    $stream_config =~ s/^/    /gm;
    $init_full =~ s/^/    /gm;
    $lua_test_full =~ s/^/    /gm;
    $http_config =~ s/^/    /gm;
    my $default_ngx_cfg = <<__END;
tweakBase: "$ALB_BASE/tweak"
nginxBase: "$ALB_BASE/nginx"
shareBase: "$ALB_BASE/share"
enableHTTP2: true
enablePrometheus: true
phase: "running"
metrics:
  port: 1936
  ipV4BindAddress: [0.0.0.0]
backlog: 2048
rootExtra: |
    env TEST_BASE;
streamExtra: |
    lua_package_path "$lua_path";
    $init_full
    $stream_config
httpExtra: |
    lua_package_path "$lua_path";
    $init_full
    $mock_backend
    $http_config
    $lua_test_full
frontends: 
   udp_82:
     port: 82
     protocol: udp
     ipV4BindAddress: [0.0.0.0]
   tcp_81:
     port: 81
     protocol: tcp
     ipV4BindAddress: [0.0.0.0]
   https_443:
     port: 443     
     protocol: https
     enableHTTP2: true
     ipV4BindAddress: [0.0.0.0]
   https_2443:
     port: 2443     
     enableHTTP2: true
     protocol: https
     ipV4BindAddress: [0.0.0.0]
   https_3443:
     port: 3443     
     protocol: https
     ipV4BindAddress: [0.0.0.0]
   http_28080:
     port: 28080     
     protocol: http
     ipV4BindAddress: [0.0.0.0]
   http_80:
     port: 80     
     protocol: http
     ipV4BindAddress: [0.0.0.0]
__END

    return $default_ngx_cfg;
}

sub patch_custom_location($block,$ngx_yaml_str) {
    if (!defined $block->custom_location_raw) {
        return $ngx_yaml_str;
    }
    my $loc = $block->custom_location_raw;
    # tgl_log("patch_custom_location custom location raw $loc");

    my $loc_ypp = YAML::PP->new;
    my $loc_yaml = $loc_ypp->load_string($loc);

    my $ngx_ypp = YAML::PP->new;
    my $ngx_yaml = $ngx_ypp->load_string($ngx_yaml_str);
    $ngx_yaml->{frontends}->{"http_80"}->{customLocation} = $loc_yaml;
    my $new_ngx_yaml = $ngx_ypp->dump_string($ngx_yaml);
    tgl_log("new_ngx_yaml $new_ngx_yaml");
    return $new_ngx_yaml;
}

sub gen_https_port_config($ports="",$ngx_yaml="") {
    if ($ports eq "") {
        return $ngx_yaml;
    }
    my $ypp = YAML::PP->new;   
    my $yaml = $ypp->load_string($ngx_yaml);
    my @port_list = split /,/, $ports;
    foreach my $port (@port_list) {
        tgl_log("add https port $port");
        $yaml->{frontends}->{"https_$port"} = {
            port => int($port),
            protocol => 'https',
            ipV4BindAddress => ['0.0.0.0']
        };
    }
    my $new_ngx_yaml = $ypp->dump_string($yaml);
    return $new_ngx_yaml;
}
return 1;
