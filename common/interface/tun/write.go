package tun

func (tun *TUN) Write(data []byte) (int, error) {
	return tun.file.Write(data)
}
