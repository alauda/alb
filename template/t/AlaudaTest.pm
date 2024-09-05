use feature 'signatures';

use t::AlaudaLib qw(gen_custom_init gen_mock_backend gen_env_only gen_http_only gen_main_config gen_ngx_tmpl_conf gen_lua_test gen_https_port_config gen_stream_only);
use strict;
use warnings;
use File::Basename;

sub t_log(@msgs) {
    warn "[t_log] @msgs";
}

sub disable_init_worker {
    my $self = shift;
    return $self->{"disable_init_worker"};
}

sub enable_nyi {
    my $self = shift;
    return $self->{"enable_nyi"};
}

sub init_worker_eval {
    my $self = shift;
    return $self->{"init_worker_eval"};
}

sub lua_test {
    my $self = shift;
    return $self->{"lua_test"};
}

sub lua_test_eval {
    my $self = shift;
    return $self->{"lua_test_eval"};
}

sub lua_test_file {
    my $self = shift;
    return $self->{"lua_test_file"};
}

sub mock_backend {
    my $self = shift;
    return $self->{"mock_backend"};
}

sub test_gen_custom_init() {
    my $block = bless {
        "disable_init_worker"=>"1"
    };
    t_log(gen_custom_init($block));
    t_log("--------------------------------");
    my $block = bless {
    };
    t_log(gen_custom_init($block));
    t_log("--------------------------------");
    my $block = bless {
        "enable_nyi"=>"1"
    };
    t_log(gen_custom_init($block));
    t_log("--------------------------------");
    my $block = bless {
        "init_worker_eval"=>"1"
    };
    t_log(gen_custom_init($block));
    t_log("--------------------------------");
    my $block = bless {
        "lua_test"=>"1"
    };
    t_log(gen_custom_init($block));
    t_log("--------------------------------");
    my $block = bless {
        "lua_test_eval"=>"1",
        "enable_nyi"=>"1"
    };
    t_log(gen_custom_init($block));
    t_log("--------------------------------");
}

sub test_gen_mock_backend() {
    my $block = bless {
    };
    t_log(gen_mock_backend($block->mock_backend));
    t_log("--------------------------------");
    my $block = bless {
        "mock_backend"=>"1880 e2e.rewrite_request.test"
    };
    t_log(gen_mock_backend($block->mock_backend));
    t_log("--------------------------------");
}

sub test_gen_lua_test() {
    my $block = bless {
    };
    t_log(gen_lua_test($block));
    t_log("--------------------------------");
    my $block = bless {
        "lua_test_file"=>"e2e.rewrite_request.test"
    };
    t_log(gen_lua_test($block));
    t_log("--------------------------------");
    my $block = bless {
        "lua_test"=>"1"
    };
    t_log(gen_lua_test($block));
    t_log("--------------------------------");
    my $block = bless {
        "lua_test_eval"=>"1"
    };
    t_log(gen_lua_test($block));
    t_log("--------------------------------");
}


sub test_gen_env_only() {
    my $mock_yaml = <<__END;
StreamExtra: |
    lua_package_path "/xx";
HttpExtra: |
    lua_package_path "/xx";
Frontends:
   https_443:
     port: 443
__END
    t_log(gen_env_only($mock_yaml));
    t_log("--------------------------------");
}

sub test_gen_http_only() {
    my $mock_yaml = <<__END;
httpExtra: |
    lua_package_path "/xx";
frontends:
   https_443:
     port: 443
     protocol: https
     ipV4BindAddress:
       - 0.0.0.1
__END
    t_log(gen_http_only($mock_yaml));
    t_log("--------------------------------");
}

sub test_gen_stream_only() {
    my $mock_yaml = <<__END;
streamExtra: |
    lua_package_path "/xx";
frontends:
   https_443:
     port: 443
     protocol: tcp
     ipV4BindAddress:
       - 0.0.0.1
__END
    t_log(gen_stream_only($mock_yaml));
    t_log("--------------------------------");
}

sub test_gen_https_port_config() {
    my $mock_yaml = <<__END;
frontends:
__END
    t_log(gen_https_port_config("123,223",$mock_yaml));
    t_log("--------------------------------");
}

sub test_gen_ngx_tmpl_conf() {
    my $mock_backend = gen_mock_backend("123 e2e.rewrite_request.test");
    t_log(gen_ngx_tmpl_conf("","","",$mock_backend,"",""));
    t_log("--------------------------------");
}

sub debug_test() {
    my $y = `cat ./template/ngx.yaml`;
    t_log(gen_https_port_config("123,223",$y));
}

sub test_gen_nginx_conf() {
    my $cf=gen_ngx_tmpl_conf("","","","","","");

    my $ypp = YAML::PP->new;   
    my $yaml = $ypp->load_string($cf);
    $yaml->{flags}{showRoot} = builtin::true;
    $yaml->{flags}{showEnv} = builtin::true;
    $yaml->{flags}{showHttp} = builtin::true;
    $yaml->{flags}{showHttpWrapper} = builtin::true;
    $yaml->{flags}{showMimeTypes} = builtin::true;
    $yaml->{flags}{showInitWorker} = builtin::true;
    $yaml->{flags}{showStream} = builtin::true;
    my $ngx_new = $ypp->dump_string($yaml);
    t_log($ngx_new);
    my $temp_file = File::Temp->new("ngx_full_XXXXX", SUFFIX => '.yaml');
    $temp_file->flush();
    my $temp_filepath = $temp_file->filename;
    print $temp_file $ngx_new;
    $temp_file->close();
    my $out = `ngx_gen < $temp_filepath`;
    $temp_file->unlink_on_destroy(1);
    t_log($out);
}

test_gen_custom_init();
test_gen_mock_backend();
test_gen_lua_test();
test_gen_env_only();
test_gen_http_only();
test_gen_stream_only();
test_gen_https_port_config();
debug_test();
test_gen_ngx_tmpl_conf();
test_gen_nginx_conf();