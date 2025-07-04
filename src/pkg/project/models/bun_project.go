// Copyright Project Harbor Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"github.com/goharbor/harbor/src/lib/bunorm"
	allowlist "github.com/goharbor/harbor/src/pkg/allowlist/models"
)

const (
	// BunProjectTable is the table name for project in Bun
	BunProjectTable = "project"
)

// BunProject holds the details of a project using Bun ORM.
// Migrated from Beego ORM to Bun ORM
type BunProject struct {
	bun.BaseModel `bun:"table:project"`

	// Beego: orm:"pk;auto;column(project_id)" -> Bun: bun:",pk,autoincrement"
	ProjectID int64 `bun:",pk,autoincrement" json:"project_id"`
	
	// Beego: orm:"column(owner_id)" -> Bun: bun:"owner_id"
	OwnerID int `bun:"owner_id" json:"owner_id"`
	
	// Beego: orm:"column(name)" -> Bun: bun:"name"
	Name string `bun:"name" json:"name" sort:"default"`
	
	// Beego: orm:"column(creation_time);auto_now_add" -> Bun: bun:",nullzero,notnull,default:current_timestamp"
	CreationTime time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"creation_time"`
	
	// Beego: orm:"column(update_time);auto_now" -> Bun: bun:",nullzero,notnull,default:current_timestamp"
	UpdateTime time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"update_time"`
	
	// Beego: orm:"column(deleted)" -> Bun: bun:"deleted"
	Deleted bool `bun:"deleted" json:"deleted"`
	
	// Beego: orm:"-" -> Bun: bun:"-"
	OwnerName    string                     `bun:"-" json:"owner_name"`
	Role         int                        `bun:"-" json:"current_user_role_id"`
	RoleList     []int                      `bun:"-" json:"current_user_role_ids"`
	RepoCount    int64                      `bun:"-" json:"repo_count"`
	Metadata     map[string]string          `bun:"-" json:"metadata"`
	CVEAllowlist allowlist.CVEAllowlist     `bun:"-" json:"cve_allowlist"`
	
	// Beego: orm:"column(registry_id)" -> Bun: bun:"registry_id"
	RegistryID int64 `bun:"registry_id" json:"registry_id"`
}

// BunNamesQuery is the Bun version of NamesQuery
type BunNamesQuery struct {
	Names      []string // the names of project
	WithPublic bool     // include the public projects
}

// GetMetadata ...
func (p *BunProject) GetMetadata(key string) (string, bool) {
	if len(p.Metadata) == 0 {
		return "", false
	}
	value, exist := p.Metadata[key]
	return value, exist
}

// SetMetadata ...
func (p *BunProject) SetMetadata(key, value string) {
	if p.Metadata == nil {
		p.Metadata = map[string]string{}
	}
	p.Metadata[key] = value
}

// IsPublic ...
func (p *BunProject) IsPublic() bool {
	public, exist := p.GetMetadata(ProMetaPublic)
	if !exist {
		return false
	}

	return isTrue(public)
}

// IsProxy returns true when the project type is proxy cache
func (p *BunProject) IsProxy() bool {
	return p.RegistryID > 0
}

// ContentTrustEnabled ...
func (p *BunProject) ContentTrustEnabled() bool {
	enabled, exist := p.GetMetadata(ProMetaEnableContentTrust)
	if !exist {
		return false
	}
	return isTrue(enabled)
}

// ContentTrustCosignEnabled ...
func (p *BunProject) ContentTrustCosignEnabled() bool {
	enabled, exist := p.GetMetadata(ProMetaEnableContentTrustCosign)
	if !exist {
		return false
	}
	return isTrue(enabled)
}

// VulPrevented ...
func (p *BunProject) VulPrevented() bool {
	prevent, exist := p.GetMetadata(ProMetaPreventVul)
	if !exist {
		return false
	}
	return isTrue(prevent)
}

// ReuseSysCVEAllowlist ...
func (p *BunProject) ReuseSysCVEAllowlist() bool {
	r, ok := p.GetMetadata(ProMetaReuseSysCVEAllowlist)
	if !ok {
		return true
	}
	return isTrue(r)
}

// Severity ...
func (p *BunProject) Severity() string {
	severity, exist := p.GetMetadata(ProMetaSeverity)
	if !exist {
		return ""
	}
	return severity
}

// AutoScan ...
func (p *BunProject) AutoScan() bool {
	auto, exist := p.GetMetadata(ProMetaAutoScan)
	if !exist {
		return false
	}
	return isTrue(auto)
}

// AutoSBOMGen ...
func (p *BunProject) AutoSBOMGen() bool {
	auto, exist := p.GetMetadata(ProMetaAutoSBOMGen)
	if !exist {
		return false
	}
	return isTrue(auto)
}

// ProxyCacheSpeed ...
func (p *BunProject) ProxyCacheSpeed() int32 {
	speed, exist := p.GetMetadata(ProMetaProxySpeed)
	if !exist {
		return 0
	}
	speedInt, err := strconv.ParseInt(speed, 10, 32)
	if err != nil {
		return 0
	}
	return int32(speedInt)
}

// BunProjectDAO provides database operations for BunProject using Bun ORM
type BunProjectDAO struct{}

// FilterByPublic applies public filter to Bun query
func (dao *BunProjectDAO) FilterByPublic(ctx context.Context, query *bun.SelectQuery, value interface{}) *bun.SelectQuery {
	subQuery := `SELECT project_id FROM project_metadata WHERE name = 'public' AND value = '%s'`
	if isTrue(value) {
		subQuery = fmt.Sprintf(subQuery, "true")
	} else {
		subQuery = fmt.Sprintf(subQuery, "false")
	}
	return query.Where(fmt.Sprintf("project_id IN (%s)", subQuery))
}

// FilterByOwner applies owner filter to Bun query
func (dao *BunProjectDAO) FilterByOwner(ctx context.Context, query *bun.SelectQuery, value interface{}) *bun.SelectQuery {
	username, ok := value.(string)
	if !ok {
		return query
	}

	return query.Where("owner_id IN (SELECT user_id FROM harbor_user WHERE username = ?)", username)
}

// FilterByMember applies member filter to Bun query
func (dao *BunProjectDAO) FilterByMember(ctx context.Context, query *bun.SelectQuery, value interface{}) *bun.SelectQuery {
	memberQuery, ok := value.(*BunMemberQuery)
	if !ok {
		return query
	}

	subQuery := fmt.Sprintf(`SELECT project_id FROM project_member WHERE entity_id = %d AND entity_type = 'u'`, memberQuery.UserID)
	if memberQuery.Role > 0 {
		subQuery = fmt.Sprintf("%s AND role = %d", subQuery, memberQuery.Role)
	}

	if memberQuery.WithPublic {
		subQuery = fmt.Sprintf("(%s) UNION (SELECT project_id FROM project_metadata WHERE name = 'public' AND value = 'true')", subQuery)
	}

	if len(memberQuery.GroupIDs) > 0 {
		var elems []string
		for _, groupID := range memberQuery.GroupIDs {
			elems = append(elems, strconv.Itoa(groupID))
		}

		tpl := "(%s) UNION (SELECT project_id FROM project_member pm, user_group ug WHERE pm.entity_id = ug.id AND pm.entity_type = 'g' AND ug.id IN (%s))"
		subQuery = fmt.Sprintf(tpl, subQuery, strings.TrimSpace(strings.Join(elems, ", ")))
	}

	return query.Where(fmt.Sprintf("project_id IN (%s)", subQuery))
}

// FilterByNames applies names filter to Bun query
func (dao *BunProjectDAO) FilterByNames(ctx context.Context, query *bun.SelectQuery, value interface{}) *bun.SelectQuery {
	namesQuery, ok := value.(*BunNamesQuery)
	if !ok {
		return query
	}

	if len(namesQuery.Names) == 0 {
		return query
	}

	// Use Bun's In() method for safer query building
	subQuery := query.NewSelect().
		Column("project_id").
		Table("project").
		Where("name IN (?)", bun.In(namesQuery.Names))

	if namesQuery.WithPublic {
		publicQuery := query.NewSelect().
			Column("project_id").
			Table("project_metadata").
			Where("name = 'public' AND value = 'true'")
		
		// Union the queries
		return query.Where("project_id IN (?) OR project_id IN (?)", subQuery, publicQuery)
	}

	return query.Where("project_id IN (?)", subQuery)
}

// BunMemberQuery is the Bun version of MemberQuery
type BunMemberQuery struct {
	UserID     int
	Role       int
	WithPublic bool
	GroupIDs   []int
}

// BunProjects is a slice of BunProject pointers
type BunProjects []*BunProject

// OwnerIDs returns all the owner ids from the projects
func (projects BunProjects) OwnerIDs() []int {
	var ownerIDs []int
	for _, project := range projects {
		ownerIDs = append(ownerIDs, project.OwnerID)
	}
	return ownerIDs
}

// Example usage of BunProject with database operations:
//
// // Get database from context
// db, err := bunorm.FromContext(ctx)
// if err != nil {
//     return err
// }
//
// // Select all projects
// var projects []BunProject
// err = db.NewSelect().Model(&projects).Scan(ctx)
//
// // Select project by ID
// var project BunProject
// err = db.NewSelect().Model(&project).Where("project_id = ?", id).Scan(ctx)
//
// // Insert new project
// project := &BunProject{Name: "test", OwnerID: 1}
// _, err = db.NewInsert().Model(project).Exec(ctx)
//
// // Update project
// _, err = db.NewUpdate().Model(&project).WherePK().Exec(ctx)
//
// // Delete project
// _, err = db.NewDelete().Model(&project).WherePK().Exec(ctx)
//
// // Complex query with filters
// dao := &BunProjectDAO{}
// query := db.NewSelect().Model(&projects)
// query = dao.FilterByPublic(ctx, query, true)
// query = dao.FilterByOwner(ctx, query, "admin")
// err = query.Scan(ctx)