Thanks to [Paul Maddox] (https://github.com/PaulMaddox) for his [example](https://github.com/awslabs/golang-deployment-pipeline) which influenced this example leveraging CodeCommit. 

# Introduction

At its core, DevOps is about resolving the tension between rapid development and ensuring stability in production. Developers and product teams want to get the latest features out into production as quickly as possible to ensure that their applications are providing the experience that their users want, and that their competitors don’t yet have. On the other hand, Operations Engineers have, historically at least, been gatekeepers against too many changes too often in production, acting as a defense to ensure application reliability. In some cases preventing the developers from releasing as quickly as they would like. 
One of our responsibilities as next-generation cloud managed service providers is to enable developers to focus on their code by providing managed application environments. They allow an entire continuous integration and delivery pipeline, including the infrastructure on which the code runs, to be rolled out as and when they are requested by product teams. Being able to do this at scale, across a large enterprise, and with multiple product teams, means it is vital that we automate this process as much as possible.

One of our responsibilities as next-generation cloud managed service providers is to enable developers to focus on their code by providing managed application environments, where an entire continuous integration and delivery pipeline, including the infrastructure on which the code runs, can be rolled out as and when they are requested by product teams. Being able to do this at scale, across a large enterprise with multiple product teams for example, means it is vital that we automate this process as much as possible. 

Thankfully, AWS provides this functionality through their Developer Tools such as [CodeCommit](http://docs.aws.amazon.com/codecommit/latest/userguide/welcome.html), [CodePipeline](http://docs.aws.amazon.com/codepipeline/latest/userguide/welcome.html), [CodeBuild](http://docs.aws.amazon.com/codebuild/latest/userguide/welcome.html), and [CodeDeploy](http://docs.aws.amazon.com/codedeploy/latest/userguide/welcome.html). In additiona, all of these services can be controlled through CloudFormation, enabling us to define these environments in code and manage changes through version control. Let's see how these services map onto DevOps components and walk through how we can define these services in Cloudformation.  

# Version Control

Version control is the basis of all software and infrastructure development. Although [CodePipeline](http://docs.aws.amazon.com/codepipeline/latest/userguide/welcome.html) will work with [Github](https://github.com/) and other 3rd party providers, [CodeCommit](http://docs.aws.amazon.com/codecommit/latest/userguide/welcome.html) has much tighter integration with other AWS services and allows you to define and control both the repositories, and access to those repositories, through Cloudformation, enabling much closer control of each environment. The Cloudformation looks like this:

    CodeRepository:
        Type: AWS::CodeCommit::Repository
        Properties:
            RepositoryName: !Ref ApplicationName
            RepositoryDescription: !Sub Repository for the ${ApplicationName} application

Here we are passing in the name of the application and using that as the name of the repository. Once created there are [some steps](http://docs.aws.amazon.com/codecommit/latest/userguide/setting-up-ssh-unixes.html) you need to follow to grant developers access to the repository you created. 

# Continuous Integration & Continuous Deployment

The CI / CD pipeline is the framework we use to enable velocity in the development process whilst maintaining stability and reliability in each environment, whether that's staging, test, or production. [CodePipeline](http://docs.aws.amazon.com/codepipeline/latest/userguide/welcome.html) allows you to define stages which can be used to pull down code from a repository, run tests, create build artifacts, deploy code and infrastructure, and many other activities. Each of these activities is a Stage, which can be made up of multiple Actions e.g. deploying to staging might involve deploying the infrastructure as well as the code. 

We first need to set up where the code is going to come from, in this instance it's going to be the CodeCommit repository we just created, this becomes our "Source" step.

    Name: Source
        Actions:
            -
                Name: CodeCommit
                ActionTypeId:
                    Category: Source
                    Owner: AWS
                    Version: 1
                    Provider: CodeCommit
                    OutputArtifacts:
                        -  Name: Source
                    Configuration:
                    RepositoryName: !Ref ApplicationName
                    BranchName: !Ref Branch

This will retrieve the code from the branch for the repository. In this case we're assuming that staging and production are both coming from the same branch, but your development flow might vary. The "Source" stage provides the output artifacts which will be used by the "Build" stage to create the built and deployable artifacts.

As Operations Engineers, we need to provide a harness that the developers can use to ensure their test and build processes can be defined in a way that makes sense for them. Historically this has been handled through a build tool such as [Jenkins](https://jenkins.io/) (or any one of the multitude of other offerings), but we want to leverage as many of the AWS services as possible. Released at Re:invent last year, the [CodeBuild](http://docs.aws.amazon.com/codebuild/latest/userguide/welcome.html) service now also provides this functionality.

Here, we're defining the CodeBuild step and indicating the source as coming from the "Source" step above. 

    Name: Build
        Actions:
            -
                Name: CodeBuild
                InputArtifacts:
                    - Name: Source
                ActionTypeId: 
                    Category: Build
                    Owner: AWS
                    Version: 1
                    Provider: CodeBuild
                OutputArtifacts:
                    - Name: Built
                Configuration: 
                    ProjectName: !Ref CodeBuild

Once we've defined the stage in CodePipeline we need to define the CodeBuild resource itself. We're building a simple Go web service in this example but CodeBuild  can also be used to build Python, Ruby, Node.js, Docker, and Android applications. 

    CodeBuild:
            Type: AWS::CodeBuild::Project
            Properties:
                Name: !Ref ApplicationName
                Description: !Sub Build project for ${ApplicationName}
                ServiceRole: !Ref CodeBuildRole
                Source:
                    Type: CODEPIPELINE
                Environment:
                    ComputeType: BUILD_GENERAL1_SMALL
                    Image: aws/codebuild/golang:1.7.3
                    Type: LINUX_CONTAINER
                    EnvironmentVariables:
                        - 
                            Name: ARTIFACT_S3_BUCKET
                            Value: !Sub ${ArtifactS3Bucket}
                Artifacts:
                    Name: !Ref ApplicationName
                    Type: CODEPIPELINE

It's important to nore that this step is flexible, allowing the developers to define the steps that are required to build the application in their codebase. As we're only building a small Go application and so the build environment is only small, for larger builds you might want to use a different [compute type](http://docs.aws.amazon.com/codebuild/latest/userguide/build-env-ref.html#build-env-ref-compute-types). This stage might also be used to build web applications through a build tool like [Grunt](https://gruntjs.com/) or [Gulp](http://gulpjs.com/) e.g. minify javascript, process SASS files or CoffeeScript, and so on. The definition of the build goes into a *buildspec.yml* file within the code repository.

The *buildspec.yml* file runs simple lint and unit tests against the code base. Assuming all of those pass then the binary is built and all the artifacts are defined, then uploaded to the S3 location ready for deployment through CodeDeploy in the "Staging" and "Production" Stages. 

## Deployment

We will be defining 2 deployment stages, production and staging. We'll only cover staging as production and staging are identical. The "Staging" stage takes the artifacts from the "Build" phase which have been uploaded to S3 and uses Cloudformation to deploy the infrastructure and CodeDeploy to deploy the application. 

### Infrastructure

We want to ensure that the staging and production environments are as identical as possible to maintain consistency. We're deploying in separate VPCs so the CIDR ranges will be different, and the instance types (and number of instances) might be different i.e. to save costs, but in terms of architecture. Here we're only dealing with staging and production, but we might want to have a separate pipeline process (possibly on a different branch) for performance testing outwith this deployment process, with the power of Cloudformation it's not issue to do this and keep our changes in sync.

    Name: DeployInfrastructure
        RunOrder: 1
        InputArtifacts:
            - Name: Built
        ActionTypeId:
            Category: Deploy
            Owner: AWS
            Version: 1
            Provider: CloudFormation
        Configuration:
            ActionMode: REPLACE_ON_FAILURE
            RoleArn: !Sub ${CodePipelineCloudFormationRole.Arn}
            Capabilities: CAPABILITY_NAMED_IAM
            StackName: !Sub ${ApplicationName}-staging
            TemplatePath: Built::cloudformation/infrastructure.yml
            TemplateConfiguration: Built::config/staging.conf
            ParameterOverrides: !Sub |
                {
                    "ApplicationName": "${ApplicationName}",
                    "EnvironmentName": "staging",
                    "ArtifactBucket": "${ArtifactBucket}",
                    "HostedZoneName": "${HostedZoneName}"
                }

The infrastructure consists of a typical web application architecture in its own [VPC](http://docs.aws.amazon.com/AmazonVPC/latest/UserGuide/VPC_Subnets.html)), with an [Auto Scaling Group](http://docs.aws.amazon.com/autoscaling/latest/userguide/AutoScalingGroup.html) in a private subnet behind an [Application Load Balancer](http://docs.aws.amazon.com/elasticloadbalancing/latest/application/introduction.html) proving inbound traffic in a public subnet, and [NAT Gateways](http://docs.aws.amazon.com/AmazonVPC/latest/UserGuide/vpc-nat-gateway.html) providing outbound traffic. An Environment specific Route53 [RecordSet](http://docs.aws.amazon.com/Route53/latest/DeveloperGuide/rrsets-working-with.html) is also created to enable easy access to the environment. 

We are leveraging Cloudformation to deploy the infrastructure, passing in the environment specific parameter configuration in */config/staging.json*. This is replicated in the production stage, passing in the production specific configuration in */config/production.json*. 

### Code

Once the infrastructure has been deployed successfully the Pipeline moves onto deploying the code to the appropriate CodeDeploy group as defined in the infrastructure cloudformation. 

     Name: DeployApplication
        RunOrder: 2
        InputArtifacts: 
            - Name: Built
                ActionTypeId:
                Category: Deploy
                Owner: AWS
                Version: 1
                Provider: CodeDeploy
                    Configuration: 
                        ApplicationName: !Ref ApplicationName
                        DeploymentGroupName: staging

We're using the artifacts from the build step that have been uploaded to S3. Once the Application has been deployed sucessfully, we move onto production. If you're not quite ready to automatically deploy to production, at this point you can introduce a [manual approval](http://docs.aws.amazon.com/codepipeline/latest/userguide/approvals.html) stage to allow someone with the required permissions to continue the pipeline process. 

# Conclusion

So there you have it, we have a set of Cloudformation templates that are able to define a fully-featured CI/CD pipeline providing everything that a developer would require for an application environment, including version control, pipeline, build, and deployment of code and infrastructure. 

In this example we've kept the pipeline, infrastructure, and code in the same repository, but the pipeline, infrastructure, and code could be separated into different repositories and the deployment of the environment itself could be further automated by bundling it into [Service Catalog](https://aws.amazon.com/documentation/servicecatalog/), or another Pipeline, to deploy an environment whenever requested by product teams. 

The infrastructure itself could be altered to leverage [Docker](https://www.docker.com/) and the [EC2 Container Service](http://docs.aws.amazon.com/AmazonECS/latest/developerguide/Welcome.html) quite easily, by replacing the ASG in the Cloudformation template with an ECS cluster and using an [Elastic Container Registry](http://docs.aws.amazon.com/AmazonECR/latest/userguide/ECR_GetStarted.html) to store the docker artifacts for deployment. 

