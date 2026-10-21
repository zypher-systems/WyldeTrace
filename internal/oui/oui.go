package oui

import "strings"

// Compact common OUI map (first 3 MAC octets, lowercase hex, no separators).
var vendors = map[string]string{
	"000c29": "VMware",
	"005056": "VMware",
	"000569": "VMware",
	"001c14": "VMware",
	"080027": "VirtualBox",
	"525400": "QEMU/KVM",
	"00163e": "Xen",
	"0003ff": "Microsoft Hyper-V",
	"00155d": "Microsoft Hyper-V",
	"0050f2": "Microsoft",
	"001dd8": "Microsoft",
	"f0d4e2": "Dell",
	"b8ac6f": "Dell",
	"001a2b": "Dell",
	"002219": "Dell",
	"d09466": "Dell",
	"3c2c30": "Dell",
	"001e4f": "Dell",
	"e4f004": "Dell",
	"b8ca3a": "Dell",
	"001b21": "Intel",
	"001e67": "Intel",
	"3c970e": "Intel",
	"a0369f": "Intel",
	"f8f21e": "Intel",
	"00e04c": "Realtek",
	"00a0c9": "Intel",
	"5c514f": "Intel",
	"001b77": "Intel",
	"a0a8cd": "Intel",
	"001cc0": "Intel",
	"3cfdfe": "Intel",
	"00000c": "Cisco",
	"00142a": "Cisco",
	"0021a0": "Cisco",
	"f4cfc5": "Cisco",
	"001b54": "Cisco",
	"001e13": "Cisco",
	"002314": "Cisco",
	"001e68": "Cisco",
	"0026ca": "Cisco",
	"046273": "Cisco",
	"e448c7": "Cisco Meraki",
	"088fc3": "Cisco Meraki",
	"002590": "Super Micro",
	"10604b": "Hewlett Packard",
	"3c4a92": "Hewlett Packard",
	"9440c9": "Hewlett Packard",
	"0016d3": "Hewlett Packard",
	"001a4b": "Hewlett Packard",
	"002264": "Hewlett Packard",
	"00215a": "Hewlett Packard",
	"00237d": "Hewlett Packard",
	"a0b3cc": "Hewlett Packard",
	"001cc4": "Hewlett Packard",
	"001e0b": "Hewlett Packard",
	"b4b52f": "Hewlett Packard",
	"18a905": "Hewlett Packard",
	"c8d3ff": "Hewlett Packard",
	"d89d67": "Hewlett Packard",
	"70f35a": "Apple",
	"acbc32": "Apple",
	"f0d1a9": "Apple",
	"3c0754": "Apple",
	"a4b197": "Apple",
	"0017f2": "Apple",
	"002312": "Apple",
	"001b63": "Apple",
	"001ec2": "Apple",
	"d8a25e": "Apple",
	"34b1eb": "Apple",
	"f01898": "Apple",
	"ac87a3": "Apple",
	"a88fd9": "Apple",
	"28c2dd": "Samsung",
	"0024e9": "Samsung",
	"001247": "Samsung",
	"001632": "Samsung",
	"0018af": "Samsung",
	"382c4a": "ASUSTek",
	"64d154": "Ubiquiti",
	"787a53": "Ubiquiti",
	"fc0a81": "Ubiquiti",
	"802aa8": "Ubiquiti",
	"24a43c": "Ubiquiti",
	"68d79a": "Ubiquiti",
	"e063da": "Ubiquiti",
	"f09fc2": "Ubiquiti",
	"18e829": "Ubiquiti",
	"0418d6": "Ubiquiti",
	"44d9e7": "Ubiquiti",
	"0014a5": "Ruckus",
	"001392": "Ruckus",
	"2c5d93": "Ruckus",
	"001d2e": "Ruckus",
	"000c42": "Routerboard",
	"4c5e0c": "Routerboard",
	"6487ff": "Routerboard",
	"6c3b6b": "Routerboard",
	"d4ca6e": "Routerboard",
	"b827eb": "Raspberry Pi",
	"dca632": "Raspberry Pi",
	"e45f01": "Raspberry Pi",
	"28cdac": "Raspberry Pi",
	"2cf432": "Espressif",
	"30aea4": "Espressif",
	"a4cf12": "Espressif",
	"840d8e": "Espressif",
	"246f28": "Espressif",
	"cc50e3": "Espressif",
	"94b97e": "Espressif",
	"3c71bf": "Espressif",
	"000f3d": "D-Link",
	"001cf0": "D-Link",
	"0015e9": "D-Link",
	"001b11": "D-Link",
	"14d64d": "D-Link",
	"c8be19": "D-Link",
	"001e58": "D-Link",
	"001346": "Netgear",
	"001b2f": "Netgear",
	"001e2a": "Netgear",
	"0024b2": "Netgear",
	"a04246": "Netgear",
	"c40415": "Netgear",
	"204e7f": "Netgear",
	"001dd3": "Arris",
	"b4fbde": "Arris",
	"0004f2": "Polycom",
}

// Lookup returns a vendor name for a MAC address, or empty string.
func Lookup(mac string) string {
	key := normalizePrefix(mac)
	if key == "" {
		return ""
	}
	return vendors[key]
}

func normalizePrefix(mac string) string {
	mac = strings.ToLower(strings.TrimSpace(mac))
	mac = strings.NewReplacer(":", "", "-", "", ".", "", " ", "").Replace(mac)
	if len(mac) < 6 {
		return ""
	}
	return mac[:6]
}

type Suggestion struct {
	Vendor         string   `json:"vendor"`
	DeviceTypeName string   `json:"device_type_name"`
	DeviceTypeHint string   `json:"device_type_hint"`
	Confidence     string   `json:"confidence"`
	Hints          []string `json:"hints"`
}

// SuggestDeviceType picks a coarse device-type name from vendor + open ports.
func SuggestDeviceType(mac string, ports []int, existingHints []string) Suggestion {
	vendor := Lookup(mac)
	portSet := map[int]bool{}
	for _, p := range ports {
		portSet[p] = true
	}
	hints := append([]string{}, existingHints...)
	if vendor != "" {
		hints = appendUnique(hints, "vendor:"+strings.ToLower(strings.ReplaceAll(vendor, " ", "_")))
	}

	name := "Unknown"
	conf := "low"
	switch {
	case strings.Contains(vendor, "Cisco") || strings.Contains(vendor, "Meraki") ||
		strings.Contains(vendor, "Ubiquiti") || strings.Contains(vendor, "Routerboard") ||
		strings.Contains(vendor, "Ruckus"):
		name, conf = "Network", "medium"
	case strings.Contains(vendor, "VMware") || strings.Contains(vendor, "VirtualBox") ||
		strings.Contains(vendor, "QEMU") || strings.Contains(vendor, "Hyper-V") ||
		strings.Contains(vendor, "Xen"):
		name, conf = "Virtual Machine", "high"
	case strings.Contains(vendor, "Raspberry") || strings.Contains(vendor, "Espressif"):
		name, conf = "IoT", "medium"
	case portSet[3389]:
		name, conf = "Workstation", "medium"
	case portSet[445] && portSet[22]:
		name, conf = "Server", "medium"
	case portSet[445]:
		name, conf = "Workstation", "low"
	case portSet[22] && (portSet[80] || portSet[443]):
		name, conf = "Server", "medium"
	case portSet[22]:
		name, conf = "Server", "low"
	case portSet[80] || portSet[443] || portSet[8080] || portSet[8443]:
		name, conf = "Server", "low"
	case vendor != "":
		name, conf = "Endpoint", "low"
	}

	return Suggestion{
		Vendor:         vendor,
		DeviceTypeName: name,
		DeviceTypeHint: name,
		Confidence:     conf,
		Hints:          hints,
	}
}

func appendUnique(in []string, v string) []string {
	for _, x := range in {
		if x == v {
			return in
		}
	}
	return append(in, v)
}
