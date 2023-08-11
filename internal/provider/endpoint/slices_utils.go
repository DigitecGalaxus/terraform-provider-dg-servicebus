package endpoint

func getUniqueElements(array []string) []string {
	seen := make(map[string]bool)
	unique := []string{}
	for _, item := range array {
		if !seen[item] {
			seen[item] = true
			unique = append(unique, item)
		}
	}

	return unique
}
