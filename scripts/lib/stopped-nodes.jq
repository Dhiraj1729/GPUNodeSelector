# Accept only a nonempty, complete inventory of this profile's stopped nodes.
if type == "object" then [.] elif type == "array" then . else [] end
| length > 0
  and ([.[].Name] | sort) == ($expected | sort)
  and all(.[]; .Host == "Stopped")
