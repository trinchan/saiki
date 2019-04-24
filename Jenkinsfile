node(){
    utils.checkout()
    def buildArg="""
        build:
            provider: docker
            images:
                - fullname: 'saiki:${GIT_COMMIT_SHORT}'
    """
    def builtImages = buildPushImage(buildArg)
    println builtImages
    icdHelmCharts.update('orchestration', 'saiki', builtImages)
    if(BRANCH_NAME != 'master'){
        return
    }
    // test the built image and commit a PR to prepare-cluster
    compose.testImageCommitPR([
        type: 'SYSIMAGE',
        chartName: 'saiki',
        images: builtImages
    ])
}