# API Docs

## General

- All endpoints support `?pretty` to pretty print the JSON.
- All listing endpoints support `?limit=<n>` to limit the number of returned objects.
- Some listing endpoints support `?brief` to hide less important fields, to make the dataset smaller when they're not needed.
- Put may have patch semantics.

## Users

**Warning**: Will probably change when AuthN/Z is implemented.

- `/users/?[user_name=<>]`: Get all users. Optionally filter by username.
- `/user/[id]`: Get/post/put/delete a single user.

## Documents

- `/document-families/`: Get all address families.
- `/document-family/`: Get/post/put/delete a single document family.
- `/documents/?[family=<>]&[shortname=<>]`: Get all documents. Optionally filtered by address family and shortname.
- `/document/<id>`: Get/post/put/delete a single document.
