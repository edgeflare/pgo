# Setup Logical Replication

[Logical replication](https://www.postgresql.org/docs/current/logical-replication.html) in PostgreSQL focuses on replicating data changes based on their identity (like a primary key) rather than copying entire blocks of data. This allows for finer control and greater flexibility compared to physical replication. It uses a publish-subscribe model.

## Container

```shell
# edgeflare/postgresql = bitnami/postgresql + wal-g
docker run -d -e POSTGRES_PASSWORD=secret -e POSTGRESQL_WAL_LEVEL=logical \
  -p 5432:5432 docker.io/edgeflare/postgresql:16
```

## Linux

```shell
## install
sudo dnf update -y && sudo dnf install -y postgresql-server postgresql-contrib
# apt upgrade && apt install -y postgresql postgresql-contrib # debian/ubuntu
sudo postgresql-setup --initdb --unit postgresql
sudo systemctl enable --now postgresql

# query to get the config file path
PGCONF=$(sudo -u postgres psql -Atc "show config_file;")

# set listen_addresses = '*' to listen on all interfaces
sudo sed -i "s/^#listen_addresses.*/listen_addresses = '*'/" "$PGCONF"
grep -q "^listen_addresses = '*'" "$PGCONF" || echo "listen_addresses = '*'" | sudo tee -a "$PGCONF"

# Ensure wal_level=logical in $PGCONF
sudo sed -i "s/^#wal_level.*/wal_level = logical/" "$PGCONF"
grep -q "^wal_level = logical" "$PGCONF" || echo "wal_level = logical" | sudo tee -a "$PGCONF"

PGDATA=$(sudo -u postgres psql -Atc "show data_directory;")
PG_HBA_CONF="$PGDATA/pg_hba.conf"

# add host entry for remote connections is present in pg_hba.conf
HOSTENTRY="host    all             all             0.0.0.0/0            scram-sha-256"

# Check if the entry exists, if not append it
if ! grep -q "^$HOSTENTRY" "$PG_HBA_CONF"; then
    echo "$HOSTENTRY" | sudo tee -a "$PG_HBA_CONF"
    echo "Entry added to $PG_HBA_CONF"
else
    echo "Entry already exists in $PG_HBA_CONF"
fi

# reload PostgreSQL for the changes to take effect
sudo systemctl restart postgresql
```

## AWS RDS

Check if `wal_level=logical` in AWS RDS in existing db-instance

```sql
SELECT name,setting FROM pg_settings WHERE name IN ('wal_level','rds.logical_replication');
-- expected output
--            name           | setting 
--  -------------------------+---------
--   rds.logical_replication | on
--   wal_level               | logical
```

### Create a publicly-accessible, free-tier-db for test/dev (know what you're doing!!)

```shell
export PGUSER=postgres
export PGPASSWORD=$(openssl rand -base64 16)
export DB_ID=free-tier-db-1

aws rds create-db-parameter-group \
  --db-parameter-group-name postgres-logical \
  --db-parameter-group-family postgres16 \
  --description "wal_logical=logical"

aws rds modify-db-parameter-group \
  --db-parameter-group-name postgres-logical \
  --parameters "ParameterName=rds.logical_replication,ParameterValue=1,ApplyMethod=pending-reboot" 

aws rds create-db-instance \
  --db-instance-identifier $DB_ID \
  --db-instance-class db.t3.micro \
  --engine postgres \
  --allocated-storage 20 \
  --master-username $PGUSER \
  --master-user-password $PGPASSWORD \
  --backup-retention-period 7 \
  --publicly-accessible \
  --storage-type gp2 \
  --db-parameter-group-name postgres-logical

aws rds wait db-instance-available --db-instance-identifier $DB_ID

export PGHOST=$(aws rds describe-db-instances --db-instance-identifier $DB_ID --query "DBInstances[0].Endpoint.Address" --output text)
export PGPORT=$(aws rds describe-db-instances --db-instance-identifier $DB_ID --query "DBInstances[0].Endpoint.Port" --output text)
export PGSSLMODE=require

export SG_ID=$(aws rds describe-db-instances --db-instance-identifier $DB_ID --query "DBInstances[0].VpcSecurityGroups[*].VpcSecurityGroupId" --output text)
aws ec2 authorize-security-group-ingress \
  --group-id $SG_ID \
  --protocol tcp \
  --port 5432 \
  --cidr 0.0.0.0/0

# aws rds modify-db-instance \
#   --db-instance-identifier $DB_ID \
#   --db-parameter-group-name postgres-logical \
#   --apply-immediately
```

### **DESTROY**
```shell
aws rds delete-db-instance \
  --db-instance-identifier $DB_ID \
  --skip-final-snapshot

aws rds wait db-instance-deleted --db-instance-identifier $DB_ID

aws rds delete-db-parameter-group --db-parameter-group-name postgres-logical
aws ec2 delete-security-group --group-id $SG_ID
```

## GCP Cloud SQL

https://cloud.google.com/sql/docs/postgres/replication/configure-logical-replication

```shell
# ensure sql apis are enabled
gcloud services list
gcloud services enable sql-component.googleapis.com
gcloud services enable sqladmin.googleapis.com
# gcloud config set project YOUR_PROJECT_ID

# create an instance
export INSTANCE_NAME=postgres16-sandbox

gcloud sql instances create $INSTANCE_NAME \
  --database-version=POSTGRES_16 \
  --cpu=2 \
  --memory=4096MB \
  --region=europe-west4 \
  --authorized-networks=0.0.0.0/0 \
  --storage-type=SSD \
  --storage-size=10GB \
  --edition=ENTERPRISE

gcloud sql users set-password postgres \
  --instance=$INSTANCE_NAME \
  --password=$PGPASSWORD

export PGHOST=$(gcloud sql instances describe $INSTANCE_NAME --format="get(ipAddresses[0].ipAddress)")

# enable wal_level=logical
gcloud sql instances patch $INSTANCE_NAME --database-flags=cloudsql.logical_decoding=on
```

```sql
CREATE USER replication_user WITH REPLICATION
IN ROLE cloudsqlsuperuser LOGIN PASSWORD 'secret';
-- or existing_user
-- ALTER USER existing_user WITH REPLICATION;
```
