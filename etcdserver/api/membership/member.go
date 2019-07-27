package membership

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"sort"
	"time"

	"github.com/iScript/etcd-cr/pkg/types"
)

// RaftAttributes represents the raft related attributes of an etcd member.
type RaftAttributes struct {
	// PeerURLs is the list of peers in the raft cluster.
	// TODO(philips): ensure these are URLs
	PeerURLs []string `json:"peerURLs"`
	// IsLearner indicates if the member is raft learner.
	IsLearner bool `json:"isLearner,omitempty"`
}

// Attributes represents all the non-raft related attributes of an etcd member.
type Attributes struct {
	Name       string   `json:"name,omitempty"`
	ClientURLs []string `json:"clientURLs,omitempty"`
}

type Member struct {
	ID types.ID `json:"id"`
	RaftAttributes
	Attributes
}

func NewMember(name string, peerURLs types.URLs, clusterName string, now *time.Time) *Member {

	return newMember(name, peerURLs, clusterName, now, false)
}

// newMember 并设置id
func newMember(name string, peerURLs types.URLs, clusterName string, now *time.Time, isLearner bool) *Member {
	m := &Member{
		RaftAttributes: RaftAttributes{
			PeerURLs:  peerURLs.StringSlice(),
			IsLearner: isLearner,
		},
		Attributes: Attributes{Name: name},
	}

	var b []byte
	sort.Strings(m.PeerURLs)
	for _, p := range m.PeerURLs {
		b = append(b, []byte(p)...)
	}

	b = append(b, []byte(clusterName)...)
	if now != nil {
		b = append(b, []byte(fmt.Sprintf("%d", now.Unix()))...)
	}

	hash := sha1.Sum(b)
	m.ID = types.ID(binary.BigEndian.Uint64(hash[:8]))
	return m

}

func (m *Member) Clone() *Member {
	if m == nil {
		return nil
	}
	mm := &Member{
		ID: m.ID,
		RaftAttributes: RaftAttributes{
			IsLearner: m.IsLearner,
		},
		Attributes: Attributes{
			Name: m.Name,
		},
	}
	if m.PeerURLs != nil {
		mm.PeerURLs = make([]string, len(m.PeerURLs))
		copy(mm.PeerURLs, m.PeerURLs)
	}
	if m.ClientURLs != nil {
		mm.ClientURLs = make([]string, len(m.ClientURLs))
		copy(mm.ClientURLs, m.ClientURLs)
	}
	return mm
}
