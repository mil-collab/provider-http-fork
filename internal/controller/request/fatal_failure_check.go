package request

import (
	"context"
	"fmt"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/responseconverter"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/jq"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const errFatalFailureCheck = "%s.Type should be either DEFAULT, CUSTOM or empty"

type fatalFailureCheck interface {
	Check(ctx context.Context, cr *v1alpha2.Request, details httpClient.HttpDetails, responseErr error) (bool, error)
}

type defaultFatalFailureCheck struct{}

func (d *defaultFatalFailureCheck) Check(ctx context.Context, cr *v1alpha2.Request, details httpClient.HttpDetails, responseErr error) (bool, error) {
	return false, nil
}

type customFatalFailureCheck struct {
	localKube client.Client
	logger    logging.Logger
	http      httpClient.Client
}

func (c *customFatalFailureCheck) Check(ctx context.Context, cr *v1alpha2.Request, details httpClient.HttpDetails, responseErr error) (bool, error) {
	logic := cr.Spec.ForProvider.FatalFailureCheck.Logic

	response := responseconverter.HttpResponseToV1alpha1Response(details.HttpResponse)

	sensitiveResponse, err := datapatcher.PatchSecretsIntoResponse(ctx, c.localKube, response, c.logger)
	if err != nil {
		return false, err
	}

	sensitiveRequestContext := requestgen.GenerateRequestContext(cr.Spec.ForProvider, sensitiveResponse)

	jqQuery := utils.NormalizeWhitespace(logic)
	sensitiveJQQuery, err := datapatcher.PatchSecretsIntoString(ctx, c.localKube, jqQuery, c.logger)
	if err != nil {
		return false, err
	}

	isFatal, err := jq.ParseBool(sensitiveJQQuery, sensitiveRequestContext)
	c.logger.Debug(fmt.Sprintf("Applying fatal failure JQ filter %s, result is %v", jqQuery, isFatal))
	if err != nil {
		return false, err
	}

	return isFatal, nil
}

var fatalFailureCheckFactoryMap = map[string]func(localKube client.Client, logger logging.Logger, http httpClient.Client) fatalFailureCheck{
	v1alpha2.ExpectedResponseCheckTypeDefault: func(localKube client.Client, logger logging.Logger, http httpClient.Client) fatalFailureCheck {
		return &defaultFatalFailureCheck{}
	},
	v1alpha2.ExpectedResponseCheckTypeCustom: func(localKube client.Client, logger logging.Logger, http httpClient.Client) fatalFailureCheck {
		return &customFatalFailureCheck{localKube: localKube, logger: logger, http: http}
	},
}

func getFatalFailureCheck(cr *v1alpha2.Request, localKube client.Client, logger logging.Logger, http httpClient.Client) fatalFailureCheck {
	if factory, ok := fatalFailureCheckFactoryMap[cr.Spec.ForProvider.FatalFailureCheck.Type]; ok {
		return factory(localKube, logger, http)
	}
	return fatalFailureCheckFactoryMap[v1alpha2.ExpectedResponseCheckTypeDefault](localKube, logger, http)
}

func runFatalFailureCheck(ctx context.Context, cr *v1alpha2.Request, details httpClient.HttpDetails, responseErr error, localKube client.Client, logger logging.Logger, http httpClient.Client) (bool, error) {
	checker := getFatalFailureCheck(cr, localKube, logger, http)
	result, err := checker.Check(ctx, cr, details, responseErr)
	if err != nil {
		return false, errors.Errorf(errFatalFailureCheck, "fatalFailureCheck")
	}
	return result, nil
}
