package display

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRTCSettingsFromEnv(t *testing.T) {
	t.Setenv(rtcUDPPortMinEnv, "30000")
	t.Setenv(rtcUDPPortMaxEnv, "30100")
	t.Setenv(rtcNATIPsEnv, "127.0.0.1, 10.0.0.10")

	svc := &Service{logger: slog.Default()}
	cfg, err := svc.resolveRTCSettings(nil)
	if err != nil {
		t.Fatalf("resolveRTCSettings returned error: %v", err)
	}
	if cfg.UDPPortMin != 30000 || cfg.UDPPortMax != 30100 {
		t.Fatalf("unexpected UDP range: %d-%d", cfg.UDPPortMin, cfg.UDPPortMax)
	}
	if len(cfg.NATIPs) != 2 || cfg.NATIPs[0] != "127.0.0.1" || cfg.NATIPs[1] != "10.0.0.10" {
		t.Fatalf("unexpected NAT IPs: %#v", cfg.NATIPs)
	}
}

func TestIsSocketReadyRequiresListener(t *testing.T) {
	path := filepath.Join(os.TempDir(), "memoh-display-ready-test.sock")
	_ = os.Remove(path)
	t.Cleanup(func() { _ = os.Remove(path) })
	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	if !isSocketReady(context.Background(), path) {
		t.Fatal("expected active unix socket to be ready")
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	if isSocketReady(context.Background(), path) {
		t.Fatal("closed unix socket file must not be reported ready")
	}
}

func TestResolveRTCSettingsRejectsPartialPortRange(t *testing.T) {
	t.Setenv(rtcUDPPortMinEnv, "30000")

	svc := &Service{logger: slog.Default()}
	if _, err := svc.resolveRTCSettings(nil); err == nil {
		t.Fatal("expected partial port range to fail")
	}
}

func TestResolveRTCSettingsRejectsInvalidNATIP(t *testing.T) {
	t.Setenv(rtcNATIPsEnv, "localhost")

	svc := &Service{logger: slog.Default()}
	if _, err := svc.resolveRTCSettings(nil); err == nil {
		t.Fatal("expected invalid NAT IP to fail")
	}
}

func TestResolveRTCSettingsUsesInferredNATIPs(t *testing.T) {
	svc := &Service{logger: slog.Default()}
	cfg, err := svc.resolveRTCSettings([]string{"100.123.2.67", "10.0.0.2"})
	if err != nil {
		t.Fatalf("resolveRTCSettings returned error: %v", err)
	}
	if len(cfg.NATIPs) != 2 || cfg.NATIPs[0] != "100.123.2.67" || cfg.NATIPs[1] != "10.0.0.2" {
		t.Fatalf("unexpected inferred NAT IPs: %#v", cfg.NATIPs)
	}
}

func TestResolveRTCSettingsLayersOptionsHostAndRequest(t *testing.T) {
	svc := &Service{
		logger:  slog.Default(),
		opts:    Options{NATIPs: []string{"203.0.113.5"}, STUNServers: []string{"stun.example.com:3478"}},
		hostIPs: []string{"10.0.0.10", "203.0.113.5"}, // intentional dup
	}
	cfg, err := svc.resolveRTCSettings([]string{"192.168.1.20"})
	if err != nil {
		t.Fatalf("resolveRTCSettings: %v", err)
	}
	want := []string{"203.0.113.5", "10.0.0.10", "192.168.1.20"}
	if len(cfg.NATIPs) != len(want) {
		t.Fatalf("unexpected NAT IPs: %#v", cfg.NATIPs)
	}
	for i, ip := range want {
		if cfg.NATIPs[i] != ip {
			t.Fatalf("NAT IP[%d]: got %s want %s", i, cfg.NATIPs[i], ip)
		}
	}
	servers := cfg.iceServers()
	if len(servers) != 1 || len(servers[0].URLs) != 1 || servers[0].URLs[0] != "stun:stun.example.com:3478" {
		t.Fatalf("unexpected ice servers: %#v", servers)
	}
}

func TestResolveRTCSettingsTCPEnabledFromOptions(t *testing.T) {
	svc := &Service{logger: slog.Default(), opts: Options{TCPPort: 50443}}
	cfg, err := svc.resolveRTCSettings(nil)
	if err != nil {
		t.Fatalf("resolveRTCSettings: %v", err)
	}
	if !cfg.TCPEnabled || cfg.TCPPort != 50443 {
		t.Fatalf("expected TCP enabled on port 50443, got %#v", cfg)
	}
}

func TestLocalHostIPsExcludesLoopback(t *testing.T) {
	ips, err := localHostIPs()
	if err != nil {
		t.Fatalf("localHostIPs: %v", err)
	}
	for _, ip := range ips {
		parsed := net.ParseIP(ip)
		if parsed == nil {
			t.Fatalf("invalid IP returned: %s", ip)
		}
		if parsed.IsLoopback() || parsed.IsLinkLocalUnicast() || parsed.IsUnspecified() {
			t.Fatalf("filtered IP leaked through: %s", ip)
		}
	}
}

func TestGStreamerArgsH264UsesX264AndH264Pay(t *testing.T) {
	args := gstreamerArgs(CodecH264, H264EncoderX264, 5901, 5004)
	if !containsString(args, "incremental=true") {
		t.Fatal("live rfbsrc must request incremental updates")
	}
	if !containsString(args, "use-copyrect=true") {
		t.Fatal("live rfbsrc must allow copyrect updates")
	}
	if !containsString(args, "do-timestamp=true") {
		t.Fatal("rfbsrc buffers must be timestamped for RTP encoding")
	}
	if !containsString(args, "x264enc") {
		t.Fatal("H264 pipeline must use x264enc")
	}
	if !containsString(args, "rtph264pay") {
		t.Fatal("H264 pipeline must use rtph264pay")
	}
}

func TestGStreamerArgsH264VAAPIUsesVAEncoder(t *testing.T) {
	args := gstreamerArgs(CodecH264, H264EncoderVAAPI, 5901, 5004)
	if !containsString(args, "vah264enc") {
		t.Fatal("VAAPI pipeline must use vah264enc")
	}
	if !containsString(args, "vapostproc") {
		t.Fatal("VAAPI pipeline must use vapostproc to upload frames into VAMemory")
	}
	if !containsString(args, "rtph264pay") {
		t.Fatal("VAAPI H264 pipeline must use rtph264pay")
	}
}

func TestGStreamerArgsH264NVENCUsesNVEncoder(t *testing.T) {
	args := gstreamerArgs(CodecH264, H264EncoderNVENC, 5901, 5004)
	if !containsString(args, "nvh264enc") {
		t.Fatal("NVENC pipeline must use nvh264enc")
	}
	if !containsString(args, "cudaupload") {
		t.Fatal("NVENC pipeline must upload frames into CUDA memory")
	}
}

func TestGStreamerArgsVP8FallbackUsesVP8Pay(t *testing.T) {
	args := gstreamerArgs(CodecVP8, H264EncoderX264, 5901, 5004)
	if !containsString(args, "vp8enc") {
		t.Fatal("VP8 pipeline must use vp8enc")
	}
	if !containsString(args, "rtpvp8pay") {
		t.Fatal("VP8 pipeline must use rtpvp8pay")
	}
}

func TestGStreamerScreenshotArgsCapturesComputerUseJPEG(t *testing.T) {
	args := gstreamerScreenshotArgs(5901, "/tmp/display.jpg")
	if !containsString(args, "num-buffers=1") {
		t.Fatal("screenshot pipeline must stop after one frame")
	}
	if !containsString(args, "videoscale") {
		t.Fatal("screenshot pipeline must scale in GStreamer")
	}
	if !containsString(args, "video/x-raw,width=1280,pixel-aspect-ratio=1/1") {
		t.Fatal("screenshot pipeline must capture a computer-use friendly desktop width without distorting aspect ratio")
	}
	if !containsString(args, "jpegenc") || !containsString(args, "quality=82") {
		t.Fatal("screenshot pipeline must encode bounded-size JPEG directly")
	}
	if !containsString(args, "location=/tmp/display.jpg") {
		t.Fatal("screenshot pipeline must write to requested path")
	}
	if !containsString(args, "incremental=false") {
		t.Fatal("screenshot pipeline must request a full frame")
	}
}

func TestLimitJPEGSizeRecompressesOversizedImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1280, 800))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 17) % 256),
				G: uint8((y * 31) % 256),
				B: uint8((x*y + y) % 256),
				A: 255,
			})
		}
	}

	var original bytes.Buffer
	if err := jpeg.Encode(&original, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode original jpeg: %v", err)
	}

	const maxBytes = 32 * 1024
	bounded, err := limitJPEGSize(original.Bytes(), maxBytes)
	if err != nil {
		t.Fatalf("limitJPEGSize returned error: %v", err)
	}
	if len(bounded) > maxBytes {
		t.Fatalf("bounded image is too large: %d > %d", len(bounded), maxBytes)
	}
	if _, err := jpeg.Decode(bytes.NewReader(bounded)); err != nil {
		t.Fatalf("bounded image must remain decodable JPEG: %v", err)
	}
}

func TestNegotiateCodecPrefersH264(t *testing.T) {
	// SDP fragment offering both H264 (PT 102) and VP8 (PT 96).
	offer := "v=0\r\n" +
		"o=- 0 0 IN IP4 127.0.0.1\r\n" +
		"s=-\r\n" +
		"t=0 0\r\n" +
		"m=video 9 UDP/TLS/RTP/SAVPF 102 96\r\n" +
		"c=IN IP4 0.0.0.0\r\n" +
		"a=rtpmap:102 H264/90000\r\n" +
		"a=rtpmap:96 VP8/90000\r\n"
	codec, err := negotiateCodec(offer, false)
	if err != nil {
		t.Fatalf("negotiateCodec returned error: %v", err)
	}
	if codec != CodecH264 {
		t.Fatalf("expected H264, got %s", codec)
	}
}

func TestNegotiateCodecFallsBackToVP8(t *testing.T) {
	offer := "v=0\r\n" +
		"o=- 0 0 IN IP4 127.0.0.1\r\n" +
		"s=-\r\n" +
		"t=0 0\r\n" +
		"m=video 9 UDP/TLS/RTP/SAVPF 96\r\n" +
		"c=IN IP4 0.0.0.0\r\n" +
		"a=rtpmap:96 VP8/90000\r\n"
	codec, err := negotiateCodec(offer, false)
	if err != nil {
		t.Fatalf("negotiateCodec returned error: %v", err)
	}
	if codec != CodecVP8 {
		t.Fatalf("expected VP8, got %s", codec)
	}
}

func TestNegotiateCodecForceVP8(t *testing.T) {
	offer := "v=0\r\n" +
		"o=- 0 0 IN IP4 127.0.0.1\r\n" +
		"s=-\r\n" +
		"t=0 0\r\n" +
		"m=video 9 UDP/TLS/RTP/SAVPF 102 96\r\n" +
		"c=IN IP4 0.0.0.0\r\n" +
		"a=rtpmap:102 H264/90000\r\n" +
		"a=rtpmap:96 VP8/90000\r\n"
	codec, err := negotiateCodec(offer, true)
	if err != nil {
		t.Fatalf("negotiateCodec returned error: %v", err)
	}
	if codec != CodecVP8 {
		t.Fatalf("expected forced VP8, got %s", codec)
	}
}

func TestNegotiateCodecForceVP8RejectsH264Only(t *testing.T) {
	offer := "v=0\r\n" +
		"o=- 0 0 IN IP4 127.0.0.1\r\n" +
		"s=-\r\n" +
		"t=0 0\r\n" +
		"m=video 9 UDP/TLS/RTP/SAVPF 102\r\n" +
		"c=IN IP4 0.0.0.0\r\n" +
		"a=rtpmap:102 H264/90000\r\n"
	if _, err := negotiateCodec(offer, true); err == nil {
		t.Fatal("force-VP8 must not silently fall back to H264")
	}
}

func TestNegotiateCodecNoMatch(t *testing.T) {
	offer := "v=0\r\n" +
		"o=- 0 0 IN IP4 127.0.0.1\r\n" +
		"s=-\r\n" +
		"t=0 0\r\n" +
		"m=video 9 UDP/TLS/RTP/SAVPF 100\r\n" +
		"c=IN IP4 0.0.0.0\r\n" +
		"a=rtpmap:100 AV1/90000\r\n"
	if _, err := negotiateCodec(offer, false); err == nil {
		t.Fatal("expected codec negotiation to fail without H264/VP8")
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestFilterIPsByCIDRWhitelist(t *testing.T) {
	ips := []string{
		"192.168.3.3",
		"10.8.0.2",
		"100.64.0.1",
		"172.17.0.1",
		"172.18.0.1",
		"203.0.113.5",
	}
	cidrs := []string{
		"192.168.0.0/16",
		"10.8.0.0/16",
		"100.64.0.0/10",
	}
	got := filterIPsByCIDR(ips, cidrs, slog.Default())
	want := []string{"192.168.3.3", "10.8.0.2", "100.64.0.1"}
	if len(got) != len(want) {
		t.Fatalf("filterIPsByCIDR len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("filterIPsByCIDR[%d] = %q, want %q (full=%v)", i, got[i], v, got)
		}
	}
}

func TestFilterIPsByCIDREmptyCIDRsPassThrough(t *testing.T) {
	ips := []string{"192.168.1.1", "172.17.0.1"}
	got := filterIPsByCIDR(ips, nil, slog.Default())
	if len(got) != len(ips) {
		t.Fatalf("expected pass-through, got %v", got)
	}
}

func TestFilterIPsByCIDRAllInvalidCIDRsPassThrough(t *testing.T) {
	ips := []string{"192.168.1.1", "172.17.0.1"}
	got := filterIPsByCIDR(ips, []string{"not-a-cidr", "also/bad"}, slog.Default())
	if len(got) != len(ips) {
		t.Fatalf("expected pass-through when all CIDRs invalid, got %v", got)
	}
}

func TestFilterIPsByCIDRIPv6(t *testing.T) {
	ips := []string{"fd7a:115c:a1e0::a834:6223", "2408:8270::1", "192.168.1.1"}
	got := filterIPsByCIDR(ips, []string{"fd7a::/16", "192.168.0.0/16"}, slog.Default())
	want := []string{"fd7a:115c:a1e0::a834:6223", "192.168.1.1"}
	if len(got) != len(want) {
		t.Fatalf("filterIPsByCIDR ipv6 = %v, want %v", got, want)
	}
}
