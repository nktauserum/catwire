package tun

func (tun *TUN) Read(buf []byte) (int, error) {
	return tun.file.Read(buf)
}
