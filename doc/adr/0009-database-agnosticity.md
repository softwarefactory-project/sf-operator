---
status: accepted
date: 2023-06-29
---

# Database Agnosticity for SF Deployments

## Context and Problem Statement

As of now the SF Operator deploys a MariaDB container and sets it up with hard-coded scripts. This is managed by the operator's controller. There is no expectation of high availability, scaling up, etc. Backup and restore aren't implemented yet but are on the roadmap to be isofunctional with SF 3.X.
While this is sufficient for development and testing, production deployments may have stricter requirements.

There is a MariaDB operator that we could use to provide more database deployment options: https://operatorhub.io/operator/mariadb-operator - as of https://softwarefactory-project.io/r/c/software-factory/sf-operator/+/28921 we have established the mariadb operator can be used on microshift.

We could also aim for true agnosticity, in the sense that it should be possible to use a database that was deployed out-of-band (OoB), even outside of the kubernetes cluster used to host the SF operator.

## Considered Options

* Deploy DB with the MariaDB operator
* Keep the current minimal deployment method
* Expect a connection secret with database information; if not present, deploy a minimal db like we currently do

## Decision Outcome

Option 2 - Keep the current minimal deployment method; but with the option to revisit this decision at a later time.

### Consequences

* Good, because there is nothing to do.

## Pros and Cons of the Options

### Use the MariaDB operator to deploy the database

We expect the mariadb operator to be a dependency of sf-operator. As such, we will add a makefile entry to install the operator like we do with cert-manager.
We will replace the currently existing code in controllers/mariadb.go to use the mariadb operator API instead. The user and database creation will be handled there for Zuul, and the zuul controller will consume a connection secret to set itself up.

* Good, because we get a lot of features for free (monitoring, backup/restore).
* Good, because we can use the operator API to interact with or create databases in a more "native" way.
* Good, because it creates a mariadb CR that has a distinct lifecycle and can be managed declaratively with kubectl.
* Good, because it simplifies complex operations like scaling up and replication. This opens up interesting possibilities, for example dedicating a DB replica with read-only access to users who would otherwise put a lot of pressure on the Zuul REST API to collect data.
* Bad, because this adds a dependency to the project. We cannot expect the specific operator we chose to be available everywhere especially if the host cluster is managed.

### Keep the current minimal deployment

* Good, because nothing needs to be done and we know it works.
* Good, because we have very low requirements for the database. In production so far, we've never had to handle high availability, replication, etc. This may be just us, however, and other, more intensive and critical deployments may require beefier databases.
* Good, because this makes the SF operator "batteries included".
* Bad, because we own this code and therefore need to maintain it.
* Bad, because we need to implement more critical functions like backups and restores.

### Expect a connection secret with database information; if not present, deploy a minimal db like we currently do

We add an optional connection secret reference in zuul-scheduler's spec. The secret shall be in the form of the connection secret managed by the mariadb operator, as seen here: https://github.com/mariadb-operator/mariadb-operator/blob/main/controllers/connection_controller.go#L189

If the secret isn't defined in the spec, create a database like we have done so far, the difference being that we handle creating the user and the database in the mariadb controller, resulting in the creation of a default connection secret bearing this info.

In every case, the zuul controller consumes this secret to finalize the scheduler's configuration.

A working proposal can be found here: https://softwarefactory-project.io/r/q/topic:db_connection_secret

* Good, because this is a commonly seen pattern for kubernetes apps. Many operators (Keycloak for example) also expect an external DB to be set up for them as a prerequisite to deploying instances. Developers on OpenShift are encouraged to use the Developer Catalog to deploy a database as needed by their app(s).
* Good, because it is a flexible and generic solution; by working with connection secrets we support external dbs, dbs created with the developer catalog, and dbs created with the mariadb operator (with the latter being a "privileged citizen" as a connection secret can be created easily)
* Good, because if we define the connection the same way than the mariadb operator does it, we can use this operator to deploy the zuul database fairly easily.
* Good, because we don't have to bother with meeting fault tolerance and high
  availability requirements, or backup and restore policies, of deployers. Managing
  the database's life cycle is their sole responsibility.
* Bad, because we need to handle the connection secret's life cycle: what to do when the secret is changed or deleted, etc.
* Bad, because we would still handle the case where the secret is absent, ie maintain
  our current mariadb controller. We could mitigate this by making the secret, and thus deploying a database, mandatory.
* Bad, because this de-couples the backup/restore of SF from the build history. One
  could argue however that we should de-couple config data from application data anyway.

## More Information

* Keycloak operator's approach to databases https://www.keycloak.org/operator/basic-deployment
* MariaDB operator's operatorhub page https://operatorhub.io/operator/mariadb-operator
* Deploying a database with the Developer Catalog https://docs.openshift.com/container-platform/4.13/applications/creating_applications/odc-creating-applications-using-developer-perspective.html#odc-using-the-developer-catalog-to-add-services-or-components_odc-creating-applications-using-developer-perspective
