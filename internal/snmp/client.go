package snmp

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Walker is the mockable SNMP surface used by enrichment.
type Walker interface {
	Walk(ctx context.Context, target string, opts Options) (*Snapshot, error)
}

// Options configures an SNMP session.
type Options struct {
	Community string
	Version   string // "2c" (default) or "3"
	Timeout   time.Duration
	// v3 fields (optional; used when Version == "3")
	UserName     string
	AuthProtocol string
	AuthPassword string
	PrivProtocol string
	PrivPassword string
}

// Snapshot is a parsed switch view.
type Snapshot struct {
	SysName    string
	Interfaces map[int]Iface // ifIndex → iface
	ARP        []ARPEntry
	LLDP       []LLDPEntry
}

type Iface struct {
	Index int
	Name  string
	Descr string
	MAC   string
}

type ARPEntry struct {
	IP        string
	MAC       string
	IfIndex   int
	IfName    string
}

type LLDPEntry struct {
	LocalIfIndex int
	LocalIfName  string
	RemoteSysName string
	RemotePortID  string
}

// Client is a pragmatic SNMPv2c UDP walker (GETNEXT) for ifTable + ipNetToMediaTable
// and optional LLDP rem tables. v3 is accepted in Options but returns a clear error
// unless a custom Walker is injected.
type Client struct{}

func New() *Client { return &Client{} }

const (
	oidSysName           = "1.3.6.1.2.1.1.5.0"
	oidIfDescr           = "1.3.6.1.2.1.2.2.1.2"
	oidIfName            = "1.3.6.1.2.1.31.1.1.1.1"
	oidIfPhysAddress     = "1.3.6.1.2.1.2.2.1.6"
	oidIPNetToMediaPhys  = "1.3.6.1.2.1.4.22.1.2"
	oidIPNetToMediaNet   = "1.3.6.1.2.1.4.22.1.3"
	oidLLDPRemSysName    = "1.0.8802.1.1.2.1.4.1.1.9"
	oidLLDPRemPortID     = "1.0.8802.1.1.2.1.4.1.1.7"
)

func (c *Client) Walk(ctx context.Context, target string, opts Options) (*Snapshot, error) {
	if strings.EqualFold(opts.Version, "3") {
		return nil, fmt.Errorf("SNMPv3 requires a configured Walker implementation; set version=2c or inject a Walker")
	}
	community := opts.Community
	if community == "" {
		community = "public"
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	host := target
	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, "161")
	}
	sess := &session{addr: host, community: community, timeout: timeout}
	snap := &Snapshot{Interfaces: map[int]Iface{}, ARP: []ARPEntry{}, LLDP: []LLDPEntry{}}

	if v, err := sess.get(ctx, oidSysName); err == nil {
		snap.SysName = string(v.value)
	}

	descrs, _ := sess.walk(ctx, oidIfDescr)
	names, _ := sess.walk(ctx, oidIfName)
	macs, _ := sess.walk(ctx, oidIfPhysAddress)
	for oid, vb := range descrs {
		idx := lastOIDIndex(oid)
		if idx <= 0 {
			continue
		}
		iface := snap.Interfaces[idx]
		iface.Index = idx
		iface.Descr = string(vb.value)
		snap.Interfaces[idx] = iface
	}
	for oid, vb := range names {
		idx := lastOIDIndex(oid)
		if idx <= 0 {
			continue
		}
		iface := snap.Interfaces[idx]
		iface.Index = idx
		iface.Name = string(vb.value)
		snap.Interfaces[idx] = iface
	}
	for oid, vb := range macs {
		idx := lastOIDIndex(oid)
		if idx <= 0 {
			continue
		}
		iface := snap.Interfaces[idx]
		iface.Index = idx
		iface.MAC = formatMAC(vb.value)
		snap.Interfaces[idx] = iface
	}

	arpPhys, _ := sess.walk(ctx, oidIPNetToMediaPhys)
	for oid, vb := range arpPhys {
		// oid ….1.2.<ifIndex>.<a>.<b>.<c>.<d>
		parts := strings.Split(oid, ".")
		if len(parts) < 5 {
			continue
		}
		ifIdx, _ := strconv.Atoi(parts[len(parts)-5])
		ip := strings.Join(parts[len(parts)-4:], ".")
		entry := ARPEntry{IP: ip, MAC: formatMAC(vb.value), IfIndex: ifIdx}
		if iface, ok := snap.Interfaces[ifIdx]; ok {
			entry.IfName = firstNonEmpty(iface.Name, iface.Descr)
		}
		snap.ARP = append(snap.ARP, entry)
	}

	lldpNames, _ := sess.walk(ctx, oidLLDPRemSysName)
	lldpPorts, _ := sess.walk(ctx, oidLLDPRemPortID)
	for oid, vb := range lldpNames {
		localIf := lldpLocalIf(oid)
		e := LLDPEntry{LocalIfIndex: localIf, RemoteSysName: string(vb.value)}
		if iface, ok := snap.Interfaces[localIf]; ok {
			e.LocalIfName = firstNonEmpty(iface.Name, iface.Descr)
		}
		if p, ok := lldpPorts[oid]; ok {
			e.RemotePortID = string(p.value)
		}
		snap.LLDP = append(snap.LLDP, e)
	}
	return snap, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func formatMAC(b []byte) string {
	if len(b) != 6 {
		return ""
	}
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4], b[5])
}

func lastOIDIndex(oid string) int {
	parts := strings.Split(oid, ".")
	if len(parts) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(parts[len(parts)-1])
	return n
}

func lldpLocalIf(oid string) int {
	// lldpRemEntry index: lldpRemTimeMark.lldpRemLocalPortNum.lldpRemIndex
	parts := strings.Split(oid, ".")
	if len(parts) < 3 {
		return 0
	}
	n, _ := strconv.Atoi(parts[len(parts)-2])
	return n
}

// --- minimal SNMPv2c GET / GETNEXT ---

type varBind struct {
	oid   string
	value []byte
}

type session struct {
	addr      string
	community string
	timeout   time.Duration
	reqID     int32
}

func (s *session) get(ctx context.Context, oid string) (varBind, error) {
	return s.roundTrip(ctx, 0xa0, oid) // GetRequest
}

func (s *session) walk(ctx context.Context, baseOID string) (map[string]varBind, error) {
	out := map[string]varBind{}
	cur := baseOID
	for i := 0; i < 512; i++ {
		vb, err := s.roundTrip(ctx, 0xa1, cur) // GetNextRequest
		if err != nil {
			if len(out) > 0 {
				return out, nil
			}
			return out, err
		}
		if vb.oid == "" || !strings.HasPrefix(vb.oid+".", baseOID+".") && vb.oid != baseOID {
			if !strings.HasPrefix(vb.oid, baseOID+".") {
				break
			}
		}
		if !strings.HasPrefix(vb.oid, baseOID+".") && vb.oid != baseOID {
			break
		}
		out[vb.oid] = vb
		cur = vb.oid
	}
	return out, nil
}

func (s *session) roundTrip(ctx context.Context, pduType byte, oid string) (varBind, error) {
	s.reqID++
	pkt, err := encodeGet(s.community, s.reqID, pduType, oid)
	if err != nil {
		return varBind{}, err
	}
	d := net.Dialer{Timeout: s.timeout}
	conn, err := d.DialContext(ctx, "udp", s.addr)
	if err != nil {
		return varBind{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(s.timeout))
	if _, err := conn.Write(pkt); err != nil {
		return varBind{}, err
	}
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		return varBind{}, err
	}
	return decodeFirstVarBind(buf[:n])
}

func encodeGet(community string, reqID int32, pduType byte, oid string) ([]byte, error) {
	oidBytes, err := encodeOID(oid)
	if err != nil {
		return nil, err
	}
	null := []byte{0x05, 0x00}
	varBind := appendTLV(0x30, append(oidBytes, null...))
	varBindList := appendTLV(0x30, varBind)
	reqIDEnc := encodeInt(int64(reqID))
	errStat := encodeInt(0)
	errIdx := encodeInt(0)
	pdu := appendTLV(pduType, concat(reqIDEnc, errStat, errIdx, varBindList))
	version := encodeInt(1) // SNMPv2c
	comm := appendTLV(0x04, []byte(community))
	return appendTLV(0x30, concat(version, comm, pdu)), nil
}

func decodeFirstVarBind(pkt []byte) (varBind, error) {
	// Very small ASN.1 walk to first OID + value octets.
	v, rest, err := parseTLV(pkt)
	if err != nil || v.tag != 0x30 {
		return varBind{}, fmt.Errorf("bad snmp response")
	}
	_, rest, err = parseTLV(rest) // version
	if err != nil {
		return varBind{}, err
	}
	_, rest, err = parseTLV(rest) // community
	if err != nil {
		return varBind{}, err
	}
	pdu, rest, err := parseTLV(rest)
	if err != nil {
		return varBind{}, err
	}
	_ = rest
	body := pdu.value
	_, body, _ = parseTLV(body) // req id
	_, body, _ = parseTLV(body) // err
	_, body, _ = parseTLV(body) // err idx
	vbl, _, err := parseTLV(body)
	if err != nil || vbl.tag != 0x30 {
		return varBind{}, fmt.Errorf("no varbind list")
	}
	vb, _, err := parseTLV(vbl.value)
	if err != nil {
		return varBind{}, err
	}
	oidTLV, vbRest, err := parseTLV(vb.value)
	if err != nil {
		return varBind{}, err
	}
	valTLV, _, err := parseTLV(vbRest)
	if err != nil {
		return varBind{}, err
	}
	oidStr, err := decodeOID(oidTLV.value)
	if err != nil {
		return varBind{}, err
	}
	return varBind{oid: oidStr, value: valTLV.value}, nil
}

type tlv struct {
	tag   byte
	value []byte
}

func parseTLV(b []byte) (tlv, []byte, error) {
	if len(b) < 2 {
		return tlv{}, nil, fmt.Errorf("short tlv")
	}
	tag := b[0]
	length, n, err := parseLength(b[1:])
	if err != nil {
		return tlv{}, nil, err
	}
	start := 1 + n
	if len(b) < start+length {
		return tlv{}, nil, fmt.Errorf("truncated tlv")
	}
	return tlv{tag: tag, value: b[start : start+length]}, b[start+length:], nil
}

func parseLength(b []byte) (int, int, error) {
	if len(b) == 0 {
		return 0, 0, fmt.Errorf("no length")
	}
	if b[0]&0x80 == 0 {
		return int(b[0]), 1, nil
	}
	n := int(b[0] & 0x7f)
	if n == 0 || n > 3 || len(b) < 1+n {
		return 0, 0, fmt.Errorf("bad length")
	}
	v := 0
	for i := 0; i < n; i++ {
		v = v<<8 | int(b[1+i])
	}
	return v, 1 + n, nil
}

func appendTLV(tag byte, value []byte) []byte {
	return append([]byte{tag}, append(encodeLength(len(value)), value...)...)
}

func encodeLength(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	if n <= 0xff {
		return []byte{0x81, byte(n)}
	}
	return []byte{0x82, byte(n >> 8), byte(n)}
}

func encodeInt(v int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(v))
	i := 0
	for i < 7 && (buf[i] == 0 && buf[i+1]&0x80 == 0 || buf[i] == 0xff && buf[i+1]&0x80 != 0) {
		i++
	}
	return appendTLV(0x02, buf[i:])
}

func encodeOID(oid string) ([]byte, error) {
	parts := strings.Split(strings.TrimPrefix(oid, "."), ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("bad oid")
	}
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		nums[i] = n
	}
	out := []byte{byte(nums[0]*40 + nums[1])}
	for _, n := range nums[2:] {
		out = append(out, encodeOIDSub(n)...)
	}
	return appendTLV(0x06, out), nil
}

func encodeOIDSub(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	var stack []byte
	stack = append(stack, byte(n&0x7f))
	n >>= 7
	for n > 0 {
		stack = append(stack, byte(n&0x7f)|0x80)
		n >>= 7
	}
	for i, j := 0, len(stack)-1; i < j; i, j = i+1, j-1 {
		stack[i], stack[j] = stack[j], stack[i]
	}
	return stack
}

func decodeOID(b []byte) (string, error) {
	if len(b) == 0 {
		return "", fmt.Errorf("empty oid")
	}
	first := int(b[0])
	nums := []int{first / 40, first % 40}
	i := 1
	for i < len(b) {
		v := 0
		for {
			if i >= len(b) {
				return "", fmt.Errorf("bad oid")
			}
			c := b[i]
			i++
			v = v<<7 | int(c&0x7f)
			if c&0x80 == 0 {
				break
			}
		}
		nums = append(nums, v)
	}
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, "."), nil
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
