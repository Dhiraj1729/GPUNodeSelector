# Slurped get output may be empty, a single object, or a List.
all(.[]; if .kind=="List" then
  all(.items[]; .metadata.labels["app.kubernetes.io/part-of"]=="gpu-node-selector")
else .metadata.labels["app.kubernetes.io/part-of"]=="gpu-node-selector" end)
