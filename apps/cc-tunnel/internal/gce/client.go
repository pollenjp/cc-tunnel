package gce

import (
	"context"
)

// GCEClient は Compute Engine API の操作を抽象化するインターフェース
type GCEClient interface {
	// CreateInstance は新しい GCE VM インスタンスを作成する
	CreateInstance(ctx context.Context, req *CreateInstanceRequest) (*Instance, error)
	// GetInstance は VM インスタンスの情報を取得する
	GetInstance(ctx context.Context, project, zone, name string) (*Instance, error)
	// DeleteInstance は VM インスタンスを削除する
	DeleteInstance(ctx context.Context, project, zone, name string) error
	// ListInstances は指定プロジェクト・ゾーンのインスタンス一覧を返す
	ListInstances(ctx context.Context, project, zone string) ([]*Instance, error)
}

// CreateInstanceRequest は VM インスタンス作成リクエストのパラメータ
type CreateInstanceRequest struct {
	Project       string
	Zone          string
	Name          string
	MachineType   string
	SourceImage   string // GCE source image (e.g. "projects/<proj>/global/images/family/cc-tunnel-vm")
	StartupScript string
	Labels        map[string]string
	Tags          []string `json:"tags,omitempty"` // network tags for firewall rules
	// ServiceAccountEmail は VM にアタッチする SA のメールアドレス。
	// 空文字の場合 GCE のデフォルト挙動 (default compute SA) になる。
	ServiceAccountEmail string
	// Subnetwork は VM をぶら下げる subnet の URL/path。
	// 空の場合は network 既定 subnet を GCE 側で自動選択するが、Private Google
	// Access が無効だと外部 IP 無しの VM は Artifact Registry に到達できないため、
	// 本番では PGA 有効な subnet を明示する必要がある。
	// 形式は "projects/<proj>/regions/<region>/subnetworks/<name>" など。
	Subnetwork string
}

// Instance は GCE VM インスタンスの情報
type Instance struct {
	Name      string
	Status    string // "RUNNING", "TERMINATED", etc.
	NetworkIP string // 内部 IP アドレス
	Labels    map[string]string
}
