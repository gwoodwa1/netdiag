package lldp

func parseArista(data []byte) (Result, error) {
	return parseText(data, "arista")
}
