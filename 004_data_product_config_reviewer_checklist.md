# Data Product Configuration Reviewer Checklist

Changes should be promoted from one environment to another environment after the work designated for a particular environment is completed. Raising MR that creates changes in two environments in parallel is not recommended.

## TOC Approval

TOC approval *is required* in the following cases:
1. When a data product is promoted to preprod or prod.

TOC approval *is **NOT** required* in the following cases:
1. When a new data product is onboarded to the platform.
1. When a data product is promoted from sandbox to dev.

## Change in Warehouse Size

Snowflake charges Red Hat based on warehouse size. Data product teams need to provide the relevant justification and approvals to upgrade the size of the warehouse. This also needs to be discussed internally in the platform team with the stakeholders before approval.

## Service Accounts

1. The `email` field in each service account must correspond to an individual's email address and not a group email address. This is needed for compliance and to have a 1:1 ownership of service accounts to individuals.
1. Make sure Service Accounts are scoped to the data product that is created.
1. Astro Service Accounts Only for PreProd and Prod.
1. Check if the [naming conventions](https://docs.google.com/document/d/1u0C-huRsQJ5HJxxD4ri01y6orsmCnySf0XeVmltC6D8/edit) are being followed.

*Note:* **TOC** approval is not needed for creation of service accounts in any environment.

## Inbound Data Product

1. Inbound data product should be used only with Fivetran_DB or Snowpipe_DB
1. A data product could request a new inbound data source that they don't "own" without approval of the person who actually brought that data into snowflake via fivetran or snowpipe. ***Need Action Item***


## Migrations

`Schemachange` migrations are executed as `ACCOUNTADMIN`, so it's vital that all SQL in migrations is verified and reviewed thoroughly before merging.

A few things to keep in mind while reviewing migrations:
1. Verify that incorrect or excessive access is not being granted.
1. Verify that [naming conventions](https://docs.google.com/document/d/1u0C-huRsQJ5HJxxD4ri01y6orsmCnySf0XeVmltC6D8/edit) are being followed.

## Consumers

- We do not need **TOC** approval for adding consumer access to a `data_product` or `service_account` or a `rover_group` across **any environment**, 
We are good as long as the data product owner approves the request.
- Consumer groups should be only granted access to Preprod and Prod [(slack thread)](https://gitlab.cee.redhat.com/dataverse/dataverse-config/dataproduct-config/-/merge_requests/756#note_15715014)

## Masking

- If the policy datatype is string, the mask value should be a string. If the datatype is float, the mask value should be a float or blank. example: mask: "cast(0.0 as float)" for float datatype.
- HASH_SHA1: Users assigned to this group can view SHA1-hashed values. This is only supported for string datatype
