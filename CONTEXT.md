# Lease Management System

This context defines the business language used to manage lease records, accounting workflows, and controlled access to them.

## Access Control

**User**:
A person who accesses the lease management system and may hold multiple roles concurrently.
_Avoid_: Account, operator

**Role**:
A named collection of permissions representing one responsibility in the lease management workflow. A user's effective permissions are the union of every role assigned to that user.
_Avoid_: User type, access level

**Role Assignment**:
The association granting a role to a user. Role assignments are authoritative when determining a user's roles.
_Avoid_: Primary role, role string

**Permission**:
Authorization to perform a named action on a category of lease management information.
_Avoid_: Capability, privilege

**Legal Entity Access**:
The maximum set of lease information a non-administrator may access, determined by the legal entity assigned to that user. Missing narrower scopes grant access to the full assigned legal entity.
_Avoid_: Tenant permission, default scope

**Data Scope**:
An optional store, region, or brand restriction that narrows a user's Legal Entity Access. A Data Scope never expands access beyond the assigned legal entity.
_Avoid_: Data permission, tenant

**Segregation of Duties**:
The requirement that final approval be performed by a different User from the creator or reviewer. Editing and review may be performed by the same User during the MVP stage.
_Avoid_: Role separation, four-eyes rule

**Administrative Override**:
An exceptional, reasoned, and audited bypass of Segregation of Duties by a System Admin.
_Avoid_: Admin exemption, superuser approval
