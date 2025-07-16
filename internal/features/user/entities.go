package user

type identity struct {
	// todo: future: i think i should allow for backing up off the identity as a file or user data, so that if a user loses their machine, they can just load an identity or user data and restore a new app on another machine to this state.
	EncPrivKey []byte `json:"encPrivKey"`
	PublicKey  []byte `json:"pubKey"`
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
}
