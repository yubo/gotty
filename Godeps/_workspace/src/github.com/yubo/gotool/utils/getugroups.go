package utils

func GetUGroups(username string) ([]uint32, error) {
	return getugroups(username)
}
