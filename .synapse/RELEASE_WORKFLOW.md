# Go Release Workflow

## Critical Rule
**Always update the version constant in code BEFORE tagging** - Go proxy caches are immutable.

## Release Steps

1. **Update version in code**
   ```go
   // cmd/synapse/main.go
   const version = "X.Y.Z"
   ```

2. **Commit the version bump**
   ```bash
   git add cmd/synapse/main.go
   git commit -m "chore: bump version to X.Y.Z"
   ```

3. **Create and push tag**
   ```bash
   git push
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```

4. **Index on Go proxy**
   ```bash
   curl -s "https://proxy.golang.org/github.com/swiftj/synapse/@v/vX.Y.Z.info"
   ```

5. **Verify installation**
   ```bash
   go install github.com/swiftj/synapse/cmd/synapse@vX.Y.Z
   synapse version
   ```

## Why This Order Matters
- Go proxy caches versions permanently once indexed
- If you tag before updating the version constant, the installed binary shows the old version
- You cannot update a tag once it's been indexed by the Go proxy
- The only fix is to create a new version number

## @latest Cache
- The `@latest` endpoint is cached and may take several minutes to update
- Direct version install (`@vX.Y.Z`) works immediately after indexing
