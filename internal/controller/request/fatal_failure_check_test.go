package request

import (
	"context"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
)

func Test_CustomFatalFailureCheck(t *testing.T) {
	type args struct {
		ctx         context.Context
		cr          *v1alpha2.Request
		details     httpClient.HttpDetails
		responseErr error
	}
	type want struct {
		result bool
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"CustomCheckPasses": {
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							FatalFailureCheck: v1alpha2.ExpectedResponseCheck{
								Type:  v1alpha2.ExpectedResponseCheckTypeCustom,
								Logic: `.response.statusCode == 400`,
							},
						},
					},
				},
				details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{StatusCode: 400},
				},
				responseErr: nil,
			},
			want: want{result: true, err: nil},
		},
		"CustomCheckFails": {
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							FatalFailureCheck: v1alpha2.ExpectedResponseCheck{
								Type:  v1alpha2.ExpectedResponseCheckTypeCustom,
								Logic: `.response.statusCode == 400`,
							},
						},
					},
				},
				details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{StatusCode: 200},
				},
				responseErr: nil,
			},
			want: want{result: false, err: nil},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			c := &customFatalFailureCheck{localKube: nil, logger: logging.NewNopLogger(), http: nil}
			got, gotErr := c.Check(tc.args.ctx, tc.args.cr, tc.args.details, tc.args.responseErr)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("Check(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("Check(...): -want result, +got result: %s", diff)
			}
		})
	}
}
