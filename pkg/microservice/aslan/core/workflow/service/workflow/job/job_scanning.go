/*
Copyright 2022 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package job

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/sonar"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/types/step"
)

type ScanningJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.ZadigScanningJobSpec
}

func (j *ScanningJob) Instantiate() error {
	j.spec = &commonmodels.ZadigScanningJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *ScanningJob) SetPreset() error {
	j.spec = &commonmodels.ZadigScanningJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec

	for _, scanning := range j.spec.Scannings {
		scanningInfo, err := commonrepo.NewScanningColl().Find(j.workflow.Project, scanning.Name)
		if err != nil {
			log.Errorf("find scanning: %s error: %v", scanning.Name, err)
			continue
		}
		scanning.Repos = mergeRepos(scanningInfo.Repos, scanning.Repos)
	}
	j.job.Spec = j.spec
	return nil
}

func (j *ScanningJob) GetRepos() ([]*types.Repository, error) {
	resp := []*types.Repository{}
	j.spec = &commonmodels.ZadigScanningJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}

	for _, scanning := range j.spec.Scannings {
		scanningInfo, err := commonrepo.NewScanningColl().Find(j.workflow.Project, scanning.Name)
		if err != nil {
			log.Errorf("find scanning: %s error: %v", scanning.Name, err)
			continue
		}
		resp = append(resp, mergeRepos(scanningInfo.Repos, scanning.Repos)...)
	}
	return resp, nil
}

func (j *ScanningJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.ZadigScanningJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.ZadigScanningJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}

		for _, scanning := range j.spec.Scannings {
			for _, argsScanning := range argsSpec.Scannings {
				if scanning.Name == argsScanning.Name {
					scanning.Repos = mergeRepos(scanning.Repos, argsScanning.Repos)
					break
				}
			}
		}
		j.job.Spec = j.spec
	}
	return nil
}

func (j *ScanningJob) MergeWebhookRepo(webhookRepo *types.Repository) error {
	j.spec = &commonmodels.ZadigScanningJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	for _, scanning := range j.spec.Scannings {
		scanning.Repos = mergeRepos(scanning.Repos, []*types.Repository{webhookRepo})
	}
	j.job.Spec = j.spec
	return nil
}

func (j *ScanningJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	logger := log.SugaredLogger()
	resp := []*commonmodels.JobTask{}

	j.spec = &commonmodels.ZadigScanningJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.job.Spec = j.spec

	for _, scanning := range j.spec.Scannings {
		scanningInfo, err := commonrepo.NewScanningColl().Find(j.workflow.Project, scanning.Name)
		if err != nil {
			return resp, fmt.Errorf("find scanning: %s error: %v", scanning.Name, err)
		}
		basicImage, err := commonrepo.NewBasicImageColl().Find(scanningInfo.ImageID)
		if err != nil {
			return resp, fmt.Errorf("find basic image: %s error: %v", scanningInfo.ImageID, err)
		}
		registries, err := commonservice.ListRegistryNamespaces("", true, logger)
		if err != nil {
			return resp, fmt.Errorf("list registries error: %v", err)
		}
		var timeout int64
		if scanningInfo.AdvancedSetting != nil {
			timeout = scanningInfo.AdvancedSetting.Timeout
		}
		jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{}
		jobTask := &commonmodels.JobTask{
			Name: jobNameFormat(scanning.Name + "-" + j.job.Name),
			Key:  strings.Join([]string{j.job.Name, scanning.Name}, "."),
			JobInfo: map[string]string{
				JobNameKey:      j.job.Name,
				"scanning_name": scanning.Name,
			},
			JobType: string(config.JobZadigScanning),
			Spec:    jobTaskSpec,
			Timeout: timeout,
			Outputs: scanningInfo.Outputs,
		}
		envs := getScanningJobVariables(scanning.Repos, taskID, j.workflow.Project, j.workflow.Name, j.workflow.DisplayName, "")
		envs = append(envs, &commonmodels.KeyVal{
			Key:   "SCANNING_NAME",
			Value: scanning.Name,
		})
		envs = append(envs, scanningInfo.Envs...)

		scanningImage := basicImage.Value
		if basicImage.ImageFrom == commonmodels.ImageFromKoderover {
			scanningImage = strings.ReplaceAll(config.ReaperImage(), "${BuildOS}", basicImage.Value)
		}
		jobTaskSpec.Properties = commonmodels.JobProperties{
			Timeout:             timeout,
			ResourceRequest:     scanningInfo.AdvancedSetting.ResReq,
			ResReqSpec:          scanningInfo.AdvancedSetting.ResReqSpec,
			ClusterID:           scanningInfo.AdvancedSetting.ClusterID,
			StrategyID:          scanningInfo.AdvancedSetting.StrategyID,
			BuildOS:             scanningImage,
			ImageFrom:           setting.ImageFromCustom,
			Envs:                envs,
			Registries:          registries,
			ShareStorageDetails: getShareStorageDetail(j.workflow.ShareStorages, scanning.ShareStorageInfo, j.workflow.Name, taskID),
		}

		if len(scanning.Repos) > 0 {
			jobTaskSpec.Properties.Envs = append(jobTaskSpec.Properties.Envs, &commonmodels.KeyVal{
				Key:   "BRANCH",
				Value: scanning.Repos[0].Branch,
			})
		}

		// init tools install step
		tools := []*step.Tool{}
		for _, tool := range scanningInfo.Installs {
			tools = append(tools, &step.Tool{
				Name:    tool.Name,
				Version: tool.Version,
			})
		}
		toolInstallStep := &commonmodels.StepTask{
			Name:     fmt.Sprintf("%s-%s", scanning.Name, "tool-install"),
			JobName:  jobTask.Name,
			StepType: config.StepTools,
			Spec:     step.StepToolInstallSpec{Installs: tools},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, toolInstallStep)
		// init git clone step
		gitStep := &commonmodels.StepTask{
			Name:     scanning.Name + "-git",
			JobName:  jobTask.Name,
			StepType: config.StepGit,
			Spec:     step.StepGitSpec{Repos: renderRepos(scanning.Repos, scanningInfo.Repos, jobTaskSpec.Properties.Envs)},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, gitStep)
		repoName := ""
		branch := ""
		if len(scanningInfo.Repos) > 0 {
			if scanningInfo.Repos[0].CheckoutPath != "" {
				repoName = scanningInfo.Repos[0].CheckoutPath
			} else {
				repoName = scanningInfo.Repos[0].RepoName
			}

			branch = scanningInfo.Repos[0].Branch
		}
		// init debug before step
		debugBeforeStep := &commonmodels.StepTask{
			Name:     scanning.Name + "-debug-before",
			JobName:  jobTask.Name,
			StepType: config.StepDebugBefore,
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, debugBeforeStep)
		// init shell step
		if scanningInfo.ScannerType == types.ScanningTypeSonar {
			shellStep := &commonmodels.StepTask{
				Name:     scanning.Name + "-shell",
				JobName:  jobTask.Name,
				StepType: config.StepShell,
				Spec: &step.StepShellSpec{
					Scripts:     append(strings.Split(replaceWrapLine(scanningInfo.Script), "\n"), outputScript(scanningInfo.Outputs)...),
					SkipPrepare: true,
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, shellStep)

			sonarInfo, err := commonrepo.NewSonarIntegrationColl().GetByID(context.TODO(), scanningInfo.SonarID)
			if err != nil {
				return resp, fmt.Errorf("failed to get sonar integration information to create scanning task, error: %s", err)

			}

			projectKey := sonar.GetSonarProjectKeyFromConfig(scanningInfo.Parameter)
			resultAddr, err := sonar.GetSonarAddressWithProjectKey(sonarInfo.ServerAddress, projectKey)
			if err != nil {
				log.Errorf("failed to get sonar address with project key, error: %s", err)
			}

			sonarLinkKeyVal := &commonmodels.KeyVal{
				Key:   "SONAR_LINK",
				Value: resultAddr,
			}
			jobTaskSpec.Properties.Envs = append(jobTaskSpec.Properties.Envs, sonarLinkKeyVal)
			jobTaskSpec.Properties.Envs = append(jobTaskSpec.Properties.Envs, &commonmodels.KeyVal{
				Key:          "SONAR_TOKEN",
				Value:        sonarInfo.Token,
				IsCredential: true,
			})

			jobTaskSpec.Properties.Envs = append(jobTaskSpec.Properties.Envs, &commonmodels.KeyVal{
				Key:   "SONAR_URL",
				Value: sonarInfo.ServerAddress,
			})

			if scanningInfo.EnableScanner {
				sonarConfig := fmt.Sprintf("sonar.login=%s\nsonar.host.url=%s\n%s", sonarInfo.Token, sonarInfo.ServerAddress, scanningInfo.Parameter)
				sonarConfig = strings.ReplaceAll(sonarConfig, "$branch", branch)
				sonarScript := fmt.Sprintf("set -e\ncd %s\ncat > sonar-project.properties << EOF\n%s\nEOF\nsonar-scanner", repoName, sonarConfig)
				sonarShellStep := &commonmodels.StepTask{
					Name:     scanning.Name + "-sonar-shell",
					JobName:  jobTask.Name,
					StepType: config.StepShell,
					Spec: &step.StepShellSpec{
						Scripts:     strings.Split(replaceWrapLine(sonarScript), "\n"),
						SkipPrepare: true,
					},
				}
				jobTaskSpec.Steps = append(jobTaskSpec.Steps, sonarShellStep)
			}

			if scanningInfo.CheckQualityGate {
				sonarChekStep := &commonmodels.StepTask{
					Name:     scanning.Name + "-sonar-check",
					JobName:  jobTask.Name,
					StepType: config.StepSonarCheck,
					Spec: &step.StepSonarCheckSpec{
						Parameter:   scanningInfo.Parameter,
						CheckDir:    repoName,
						SonarToken:  sonarInfo.Token,
						SonarServer: sonarInfo.ServerAddress,
					},
				}
				jobTaskSpec.Steps = append(jobTaskSpec.Steps, sonarChekStep)
			}
		} else {
			shellStep := &commonmodels.StepTask{
				Name:     scanning.Name + "-shell",
				JobName:  jobTask.Name,
				StepType: config.StepShell,
				Spec: &step.StepShellSpec{
					Scripts: append(strings.Split(replaceWrapLine(scanningInfo.Script), "\n"), outputScript(scanningInfo.Outputs)...),
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, shellStep)
		}
		// init debug after step
		debugAfterStep := &commonmodels.StepTask{
			Name:     scanning.Name + "-debug-after",
			JobName:  jobTask.Name,
			StepType: config.StepDebugAfter,
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, debugAfterStep)
		resp = append(resp, jobTask)
	}
	j.job.Spec = j.spec
	return resp, nil
}

func (j *ScanningJob) LintJob() error {
	return nil
}

func (j *ScanningJob) GetOutPuts(log *zap.SugaredLogger) []string {
	resp := []string{}
	j.spec = &commonmodels.ZadigScanningJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return resp
	}
	scanningNames := []string{}
	for _, scanning := range j.spec.Scannings {
		scanningNames = append(scanningNames, scanning.Name)
	}
	scanningInfos, _, err := commonrepo.NewScanningColl().List(&commonrepo.ScanningListOption{ScanningNames: scanningNames, ProjectName: j.workflow.Project}, 0, 0)
	if err != nil {
		log.Errorf("list scanning info failed: %v", err)
		return resp
	}
	for _, scanningInfo := range scanningInfos {
		jobKey := strings.Join([]string{j.job.Name, scanningInfo.Name}, ".")
		resp = append(resp, getOutputKey(jobKey, scanningInfo.Outputs)...)
	}
	return resp
}

func getScanningJobVariables(repos []*types.Repository, taskID int64, project, workflowName, workflowDisplayName, infrastructure string) []*commonmodels.KeyVal {
	ret := []*commonmodels.KeyVal{}

	// basic envs
	ret = append(ret, PrepareDefaultWorkflowTaskEnvs(project, workflowName, workflowDisplayName, infrastructure, taskID)...)
	// repo envs
	ret = append(ret, getReposVariables(repos)...)

	return ret
}
