.PHONY: format 
format:
	@helm template --debug ./cloud-migrator

.PHONY: dependency
dependency:
	@cd ./cloud-migrator && helm dependency update 
	@cd ./cloud-migrator/charts/cb-spider && helm dependency update
	@cd ./cloud-migrator/charts/cb-tumblebug && helm dependency update
	@cd ./cloud-migrator/charts/cm-beetle && helm dependency update
	@cd ./cloud-migrator/charts/cm-honeybee && helm dependency update
	@cd ./cloud-migrator/charts/cm-cicada && helm dependency update
	@cd ./cloud-migrator/charts/cm-grasshopper && helm dependency update
	@cd ./cloud-migrator/charts/cm-ant && helm dependency update
	@cd ./cloud-migrator/charts/cm-butterfly && helm dependency update