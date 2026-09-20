#!/bin/bash

# Add frontend validation
sed -i '/const handleSubmit = async (e: React.FormEvent) => {/a\
    e.preventDefault();\
    setErrorMessage(null);\
    \
    const cleanName = name.trim();\
    if (cleanName && !/^(\\*|@|[a-zA-Z0-9_]([a-zA-Z0-9-_]{0,61}[a-zA-Z0-9_])?(\\.[a-zA-Z0-9_]([a-zA-Z0-9-_]{0,61}[a-zA-Z0-9_])?)*)$/.test(cleanName)) {\
      setErrorMessage("Format nama record tidak valid (hostname/wildcard/@)");\
      return;\
    }\
    \
    const cleanValue = value.trim();\
    if (!cleanValue) {\
      setErrorMessage("Value tidak boleh kosong");\
      return;\
    }\
    \
    if (type === "A" && !/^(?:[0-9]{1,3}\\.){3}[0-9]{1,3}$/.test(cleanValue)) {\
      setErrorMessage("Tipe A memerlukan alamat IPv4 yang valid");\
      return;\
    }\
    if (type === "AAAA" && !/^[0-9a-fA-F:]+$/.test(cleanValue)) {\
      setErrorMessage("Tipe AAAA memerlukan alamat IPv6 yang valid");\
      return;\
    }\
    if (["CNAME", "NS", "PTR"].includes(type) && !/^([a-zA-Z0-9_]([a-zA-Z0-9-_]{0,61}[a-zA-Z0-9_])?(\\.[a-zA-Z0-9_]([a-zA-Z0-9-_]{0,61}[a-zA-Z0-9_])?)*)\\.?$/.test(cleanValue)) {\
      setErrorMessage(`Tipe ${type} memerlukan format domain yang valid`);\
      return;\
    }\
' src/components/dashboard/RecordManager.tsx

