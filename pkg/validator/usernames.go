package validator

var ReservedUsernames = []string{
	// system / roles
	"admin",
	"administrator",
	"root",
	"system",
	"sys",
	"staff",
	"moderator",
	"mod",
	"owner",
	"operator",

	// auth / accounts
	"support",
	"help",
	"helpdesk",
	"service",
	"services",
	"security",
	"auth",
	"login",
	"logout",
	"register",
	"signup",
	"signin",

	// meta / platform
	"api",
	"status",
	"about",
	"contact",
	"pricing",
	"terms",
	"privacy",
	"policy",
	"legal",
	"blog",
	"news",

	// generic traps
	"null",
	"undefined",
	"true",
	"false",
	"test",
	"guest",
	"anonymous",
}
