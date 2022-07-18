package main

type UserTable struct {
	Id        int    `gorm:"auto_increment;primary_key;" json:"userId"`
	Username  string `gorm:"unique_index;not null;size:255" json:"username"`
	Password  string `gorm:"not null;size:255" json:"password"`
	Name      string `gorm:"size:255" json:"name"`
	Email     string `gorm:"size:255" json:"email"`
	Authority int    `json:"authority"`
}

type UserPostDTO struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

type UserLoginDTO struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserIdAuthorityPrincipal struct {
	UserId    string
	Authority string
}
