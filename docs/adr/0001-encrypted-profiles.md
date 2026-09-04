# Encrypted profiles, not OS keyring

Profiles hold a database password, so they cannot sit on disk as JSON. AES-256-GCM wraps each file. The key comes from `JUSTDB_PROFILES_KEY` when set, otherwise a 32-byte file next to the ciphertext (`{dataDir}/profiles/key`, mode 0600).

EasyPanel and Docker have no user session keyring. An env var or a key file on the data volume is what those hosts can actually keep. The env var is the stronger option: the volume then has ciphertext only.
