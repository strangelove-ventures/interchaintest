package thorchain

type VersionOutput struct {
	Version string `json:"version"`
}

type NodeAccountPubKeySet struct {
	Secp256k1 string `json:"secp256k1"`
	Ed25519 string `json:"ed25519"`
}

type NodeAccount struct {
	NodeAddress string `json:"node_address"`
	Version string `json:"version"`
	IpAddress string `json:"ip_address"`
	Status string `json:"status"`
	Bond string `json:"bond"`
	ActiveBlockHeight string `json:"active_block_height"`
	BondAddress string `json:"bond_address"`
	SignerMembership []string `json:"signer_membership"`
	ValidatorConsPubKey string `json:"validator_cons_pub_key"`
	PubKeySet NodeAccountPubKeySet `json:"pub_key_set"`
}

// ProtoMessage is implemented by generated protocol buffer messages.
// Pulled from github.com/cosmos/gogoproto/proto.
type ProtoMessage interface {
	Reset()
	String() string
	ProtoMessage()
}

type ParamChange struct {
	Subspace string `json:"subspace"`
	Key      string `json:"key"`
	Value    any    `json:"value"`
}

type BuildDependency struct {
	Parent  string `json:"parent"`
	Version string `json:"version"`

	IsReplacement      bool   `json:"is_replacement"`
	Replacement        string `json:"replacement"`
	ReplacementVersion string `json:"replacement_version"`
}

type BinaryBuildInformation struct {
	Name             string            `json:"name"`
	ServerName       string            `json:"server_name"`
	Version          string            `json:"version"`
	Commit           string            `json:"commit"`
	BuildTags        string            `json:"build_tags"`
	Go               string            `json:"go"`
	BuildDeps        []BuildDependency `json:"build_deps"`
	CosmosSdkVersion string            `json:"cosmos_sdk_version"`
}

type BankMetaData struct {
	Metadata struct {
		Description string `json:"description"`
		DenomUnits  []struct {
			Denom    string   `json:"denom"`
			Exponent int      `json:"exponent"`
			Aliases  []string `json:"aliases"`
		} `json:"denom_units"`
		Base    string `json:"base"`
		Display string `json:"display"`
		Name    string `json:"name"`
		Symbol  string `json:"symbol"`
		URI     string `json:"uri"`
		URIHash string `json:"uri_hash"`
	} `json:"metadata"`
}

type QueryDenomAuthorityMetadataResponse struct {
	AuthorityMetadata DenomAuthorityMetadata `protobuf:"bytes,1,opt,name=authority_metadata,json=authorityMetadata,proto3" json:"authority_metadata" yaml:"authority_metadata"`
}

type DenomAuthorityMetadata struct {
	// Can be empty for no admin, or a valid address
	Admin string `protobuf:"bytes,1,opt,name=admin,proto3" json:"admin,omitempty" yaml:"admin"`
}
