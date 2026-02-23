package models

// ExtractPackageVersion gets the package_version from a node's metadata map.
func ExtractPackageVersion(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	prov, ok := metadata["provenance"].(map[string]interface{})
	if !ok {
		return ""
	}
	version, _ := prov["package_version"].(string)
	return version
}

// ExtractPackageName gets the package name from a node's metadata map.
func ExtractPackageName(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	prov, ok := metadata["provenance"].(map[string]interface{})
	if !ok {
		return ""
	}
	name, _ := prov["package"].(string)
	return name
}
