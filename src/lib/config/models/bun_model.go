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
	"time"
	"github.com/uptrace/bun"
)

// BunConfigEntry is the Bun ORM version of ConfigEntry
// Migrated from Beego ORM to Bun ORM
type BunConfigEntry struct {
	bun.BaseModel `bun:"table:properties"`

	// Beego: orm:"pk;auto;column(id)" -> Bun: bun:",pk,autoincrement"
	ID int64 `bun:",pk,autoincrement" json:"-"`
	
	// Beego: orm:"column(k)" -> Bun: bun:"k"
	Key string `bun:"k" json:"k"`
	
	// Beego: orm:"column(v)" -> Bun: bun:"v"
	Value string `bun:"v" json:"v"`
}

// Migration mapping documentation:
//
// Beego ORM Tag                    | Bun ORM Tag                    | Description
// --------------------------------|--------------------------------|------------------
// orm:"pk;auto;column(id)"        | bun:",pk,autoincrement"        | Primary key auto increment
// orm:"column(name)"              | bun:"name"                     | Column name mapping
// orm:"auto_now_add"              | bun:",nullzero,notnull,default:current_timestamp" | Created timestamp
// orm:"auto_now"                  | bun:",nullzero,notnull,default:current_timestamp" | Updated timestamp  
// orm:"-"                         | bun:"-"                        | Ignore field
// orm:"size(255)"                 | bun:",type:varchar(255)"       | Field type specification
// orm:"null"                      | bun:",nullzero"                | Allow null values
// orm:"unique"                    | bun:",unique"                  | Unique constraint
// TableName() method              | bun:"table:table_name"         | Table name in BaseModel tag

// Example of a more complex model migration:
type BunExampleModel struct {
	bun.BaseModel `bun:"table:example_table"`

	// Primary key with auto increment
	ID int64 `bun:",pk,autoincrement" json:"id"`
	
	// Regular string field with column name
	Name string `bun:"name,notnull" json:"name"`
	
	// Email with unique constraint
	Email string `bun:"email,unique,notnull" json:"email"`
	
	// Optional field that can be null
	Description string `bun:"description,nullzero" json:"description,omitempty"`
	
	// Timestamp fields
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"updated_at"`
	
	// Fields ignored by ORM (computed or temporary)
	TempData string `bun:"-" json:"-"`
}

// Key differences between Beego and Bun:
//
// 1. Table Name:
//    - Beego: TableName() method
//    - Bun: bun:"table:table_name" tag on bun.BaseModel
//
// 2. Primary Key:
//    - Beego: orm:"pk;auto"
//    - Bun: bun:",pk,autoincrement"
//
// 3. Column Names:
//    - Beego: orm:"column(name)"
//    - Bun: bun:"name"
//
// 4. Timestamps:
//    - Beego: orm:"auto_now_add" or orm:"auto_now"
//    - Bun: bun:",nullzero,notnull,default:current_timestamp"
//
// 5. Null Values:
//    - Beego: orm:"null"
//    - Bun: bun:",nullzero"
//
// 6. Ignore Fields:
//    - Beego: orm:"-"
//    - Bun: bun:"-"