# Never delete shared credentials/clusters or a conflicting project identity.
type=="object"
and ([.contexts[]? | select(.name==$p)]|length)<=1
and ([.users[]? | select(.name==$p)]|length)<=1
and ([.clusters[]? | select(.name==$p)]|length)<=1
and all(.contexts[]?;
  if .name==$p then .context.cluster==$p and .context.user==$p
  else .context.cluster!=$p and .context.user!=$p end)
and all(.users[]? | select(.name==$p);
  .user["client-certificate"]==($profileDir+"/client.crt")
  and .user["client-key"]==($profileDir+"/client.key"))
and all(.clusters[]? | select(.name==$p);
  (.cluster.server==$server or (.cluster.server|test("^https://127\\.0\\.0\\.1:[0-9]+$"))))
