package netbox

import (
	"github.com/fbreckle/go-netbox/netbox/client"
	"github.com/fbreckle/go-netbox/netbox/client/virtualization"
	"github.com/fbreckle/go-netbox/netbox/models"
	"github.com/go-openapi/runtime"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"strconv"
)

func resourceNetboxPrimaryIP() *schema.Resource {
	return &schema.Resource{
		Create: resourceNetboxPrimaryIPCreate,
		Read:   resourceNetboxPrimaryIPRead,
		Update: resourceNetboxPrimaryIPUpdate,
		Delete: resourceNetboxPrimaryIPDelete,

		Schema: map[string]*schema.Schema{
			"virtual_machine_id": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
			},
			"ip_address_id": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
			},
		},
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
	}
}

func resourceNetboxPrimaryIPCreate(d *schema.ResourceData, m interface{}) error {
	d.SetId(strconv.Itoa(d.Get("virtual_machine_id").(int)))

	return resourceNetboxPrimaryIPUpdate(d, m)
}

func resourceNetboxPrimaryIPRead(d *schema.ResourceData, m interface{}) error {
	api := m.(*client.NetBox)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	params := virtualization.NewVirtualizationVirtualMachinesReadParams().WithID(id)

	res, err := api.Virtualization.VirtualizationVirtualMachinesRead(params, nil)
	if err != nil {
		errorcode := err.(*runtime.APIError).Response.(runtime.ClientResponse).Code()
		if errorcode == 404 {
			// If the ID is updated to blank, this tells Terraform the resource no longer exists (maybe it was destroyed out of band). Just like the destroy callback, the Read function should gracefully handle this case. https://www.terraform.io/docs/extend/writing-custom-providers.html
			d.SetId("")
			return nil
		}
		return err
	}

	if res.GetPayload().PrimaryIp4 != nil {
		d.Set("ip_address_id", res.GetPayload().PrimaryIp4.ID)
	} else {
		// if the vm exists, but has no primary ip, consider this element deleted
		d.SetId("")
		return nil
	}
	d.Set("virtual_machine_id", res.GetPayload().ID)
	return nil
}

func resourceNetboxPrimaryIPUpdate(d *schema.ResourceData, m interface{}) error {
	api := m.(*client.NetBox)

	virtualMachineID := int64(d.Get("virtual_machine_id").(int))
	IPAddressID := int64(d.Get("ip_address_id").(int))

	// because the go-netbox library does not have patch support atm, we have to get the whole object and re-put it

	// first, get the vm
	readParams := virtualization.NewVirtualizationVirtualMachinesReadParams().WithID(virtualMachineID)
	res, err := api.Virtualization.VirtualizationVirtualMachinesRead(readParams, nil)
	if err != nil {
		return err
	}

	vm := res.GetPayload()

	// then update the FULL vm with ALL tracked attributes
	data := models.WritableVirtualMachineWithConfigContext{}
	data.Name = vm.Name
	data.Cluster = &vm.Cluster.ID
	data.Tags = vm.Tags
	data.Comments = vm.Comments
	data.Memory = vm.Memory
	data.Vcpus = vm.Vcpus
	data.Disk = vm.Disk

	if vm.Platform != nil {
		data.Platform = &vm.Platform.ID
	}

	if vm.Tenant != nil {
		data.Tenant = &vm.Tenant.ID
	}

	if vm.Role != nil {
		data.Role = &vm.Role.ID
	}

	// unset primary ip address if -1 is passed as id
	if IPAddressID == -1 {
		data.PrimaryIp4 = nil
	} else {
		data.PrimaryIp4 = &IPAddressID
	}

	updateParams := virtualization.NewVirtualizationVirtualMachinesUpdateParams().WithID(virtualMachineID).WithData(&data)

	_, err = api.Virtualization.VirtualizationVirtualMachinesUpdate(updateParams, nil)
	if err != nil {
		return err
	}
	return resourceNetboxPrimaryIPRead(d, m)
}

func resourceNetboxPrimaryIPDelete(d *schema.ResourceData, m interface{}) error {
	// Set ip_address_id to minus one and go to update. Update will set nil
	d.Set("ip_address_id", -1)
	return resourceNetboxPrimaryIPUpdate(d, m)
}
